//
// Copyright (c) 2019-2022 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package status

import (
	"context"
	"fmt"
	"strings"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var containerFailureStateReasons = []string{
	"CrashLoopBackOff",
	"ImagePullBackOff",
	"CreateContainerError",
	"RunContainerError",
}

// unrecoverablePodEventReasons contains Kubernetes events that should fail workspace startup
// if they occur related to a workspace pod. Events are stored as a map with event names as keys
// and values representing the threshold of how many times we can see an event before it is considered
// unrecoverable.
var unrecoverablePodEventReasons = map[string]int32{
	"FailedPostStartHook":   1,
	"FailedMount":           3,
	"FailedScheduling":      1,
	"FailedCreate":          1,
	"ReplicaSetCreateError": 1,
}

var unrecoverableDeploymentConditionReasons = []string{
	"FailedCreate",
}

func CheckDeploymentStatus(deployment *appsv1.Deployment) (ready bool) {
	return deployment.Status.ReadyReplicas > 0
}

func CheckDeploymentConditions(deployment *appsv1.Deployment) (healthy bool, errorMsg string) {
	conditions := deployment.Status.Conditions
	for _, condition := range conditions {
		for _, unrecoverableReason := range unrecoverableDeploymentConditionReasons {
			if condition.Reason == unrecoverableReason {
				return false, fmt.Sprintf("Detected unrecoverable deployment condition: %s %s", condition.Reason, condition.Message)
			}
		}
	}
	return true, ""
}

// checkPodsState checks if workspace-related pods are in an unrecoverable state. A pod is considered to be unrecoverable
// if it has a container with one of the containerFailureStateReasons states, or if an unrecoverable event (with reason
// matching unrecoverablePodEventReasons) has the pod as the involved object.
// Returns optional message with detected unrecoverable state details
//         error if any happens during check
func CheckPodsState(workspaceID string, namespace string, labelSelector k8sclient.MatchingLabels,
	clusterAPI sync.ClusterAPI, config *controllerv1alpha1.OperatorConfiguration) (stateMsg string, checkFailure error) {
	podList := &corev1.PodList{}
	if err := clusterAPI.Client.List(context.TODO(), podList, k8sclient.InNamespace(namespace), labelSelector); err != nil {
		return "", err
	}

	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			ok, reason := CheckContainerStatusForFailure(&containerStatus, config)
			if !ok {
				return fmt.Sprintf("Container %s has state %s", containerStatus.Name, reason), nil
			}
		}
		for _, initContainerStatus := range pod.Status.InitContainerStatuses {
			ok, reason := CheckContainerStatusForFailure(&initContainerStatus, config)
			if !ok {
				return fmt.Sprintf("Init Container %s has state %s", initContainerStatus.Name, reason), nil
			}
		}
		if msg, err := CheckPodEvents(&pod, workspaceID, clusterAPI, config); err != nil || msg != "" {
			return msg, err
		}
	}
	return "", nil
}

func CheckPodEvents(pod *corev1.Pod, workspaceID string, clusterAPI sync.ClusterAPI, config *controllerv1alpha1.OperatorConfiguration) (msg string, err error) {
	evs := &corev1.EventList{}
	selector, err := fields.ParseSelector(fmt.Sprintf("involvedObject.name=%s", pod.Name))
	if err != nil {
		return "", fmt.Errorf("failed to parse field selector: %s", err)
	}
	if err := clusterAPI.Client.List(clusterAPI.Ctx, evs, k8sclient.InNamespace(pod.Namespace), k8sclient.MatchingFieldsSelector{Selector: selector}); err != nil {
		return "", fmt.Errorf("failed to list events in namespace %s: %w", pod.Namespace, err)
	}
	for _, ev := range evs.Items {
		if ev.InvolvedObject.Kind != "Pod" {
			continue
		}

		// On OpenShift, it's possible see "FailedMount" events when using a routingClass that depends on the service-ca
		// operator. To avoid this, we always ignore FailedMount events if the message refers to the DWO-provisioned volume
		if infrastructure.IsOpenShift() &&
			ev.Reason == "FailedMount" &&
			strings.Contains(ev.Message, common.ServingCertVolumeName(common.ServiceName(workspaceID))) {
			continue
		}

		if maxCount, isUnrecoverableEvent := unrecoverablePodEventReasons[ev.Reason]; isUnrecoverableEvent {
			if !checkIfUnrecoverableEventIgnored(ev.Reason, config) && ev.Count >= maxCount {
				var msg string
				if ev.Count > 1 {
					msg = fmt.Sprintf("Detected unrecoverable event %s %d times: %s.", ev.Reason, ev.Count, ev.Message)
				} else {
					msg = fmt.Sprintf("Detected unrecoverable event %s: %s.", ev.Reason, ev.Message)
				}
				return msg, nil
			}
		}
	}
	return "", nil
}

func CheckContainerStatusForFailure(containerStatus *corev1.ContainerStatus, config *controllerv1alpha1.OperatorConfiguration) (ok bool, reason string) {
	if containerStatus.State.Waiting != nil {
		for _, failureReason := range containerFailureStateReasons {
			if containerStatus.State.Waiting.Reason == failureReason {
				return checkIfUnrecoverableEventIgnored(containerStatus.State.Waiting.Reason, config), containerStatus.State.Waiting.Reason
			}
		}
	}

	if containerStatus.State.Terminated != nil {
		for _, failureReason := range containerFailureStateReasons {
			if containerStatus.State.Terminated.Reason == failureReason {
				return checkIfUnrecoverableEventIgnored(containerStatus.State.Terminated.Reason, config), containerStatus.State.Terminated.Reason
			}
		}
	}
	return true, ""
}

func checkIfUnrecoverableEventIgnored(reason string, config *controllerv1alpha1.OperatorConfiguration) (ignored bool) {
	for _, ignoredReason := range config.Workspace.IgnoredUnrecoverableEvents {
		if ignoredReason == reason {
			return true
		}
	}
	return false
}
