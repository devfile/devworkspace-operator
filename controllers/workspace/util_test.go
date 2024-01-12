// Copyright (c) 2019-2024 Red Hat, Inc.
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

package controllers_test

import (
	"fmt"
	"path"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclock "k8s.io/utils/clock"
)

const (
	timeout  = 10 * time.Second
	interval = 250 * time.Millisecond
)

var clock kubeclock.Clock = &kubeclock.RealClock{}

func createDevWorkspace(name, fromFile string) {
	By("Loading DevWorkspace from test file")
	devworkspace := &dw.DevWorkspace{}
	Expect(loadObjectFromFile(name, devworkspace, fromFile)).Should(Succeed())

	By("Creating DevWorkspace on cluster")
	Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
	createdDW := &dw.DevWorkspace{}
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, namespacedName(devWorkspaceName, testNamespace), createdDW); err != nil {
			return false
		}
		return createdDW.Status.DevWorkspaceId != ""
	}, 10*time.Second, 250*time.Millisecond).Should(BeTrue())
}

func createStartedDevWorkspace(name, fromFile string) {
	createDevWorkspace(name, fromFile)
	devworkspace := getExistingDevWorkspace(name)
	workspaceID := devworkspace.Status.DevWorkspaceId

	By("Manually making Routing ready to continue")
	markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

	By("Setting the deployment to have 1 ready replica")
	markDeploymentReady(common.DeploymentName(workspaceID))

	currDW := &dw.DevWorkspace{}
	Eventually(func() (dw.DevWorkspacePhase, error) {
		if err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW); err != nil {
			return "", err
		}
		GinkgoWriter.Printf("Waiting for DevWorkspace to enter running phase -- Phase: %s, Message %s\n", currDW.Status.Phase, currDW.Status.Message)
		return currDW.Status.Phase, nil
	}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusRunning), "Workspace did not enter Running phase before timeout")
}

func getExistingDevWorkspace(name string) *dw.DevWorkspace {
	By(fmt.Sprintf("Getting existing DevWorkspace %s", name))
	devworkspace := &dw.DevWorkspace{}
	dwNN := namespacedName(name, testNamespace)
	Eventually(func() (string, error) {
		if err := k8sClient.Get(ctx, dwNN, devworkspace); err != nil {
			return "", err
		}
		return devworkspace.Status.DevWorkspaceId, nil
	}, timeout, interval).Should(Not(BeEmpty()))
	return devworkspace
}

// deleteDevWorkspace forces a DevWorkspace to be deleted by removing all finalizers
func deleteDevWorkspace(name string) {
	dwNN := namespacedName(name, testNamespace)
	dw := &dw.DevWorkspace{}
	dw.Name = name
	dw.Namespace = testNamespace
	// Do nothing if already deleted
	err := k8sClient.Delete(ctx, dw)
	if k8sErrors.IsNotFound(err) {
		return
	}
	Expect(err).Should(BeNil())
	err = k8sClient.Get(ctx, dwNN, dw)
	if err != nil {
		Expect(k8sErrors.IsNotFound(err)).Should(BeTrue(), "Unexpected error when deleting DevWorkspace: %s", err)
		return
	}
	isNotFound := false
	Eventually(func() error {
		err := k8sClient.Get(ctx, dwNN, dw)
		if err != nil {
			if k8sErrors.IsNotFound(err) {
				isNotFound = true
				return nil
			}
			return err
		}
		dw.Finalizers = nil
		return k8sClient.Update(ctx, dw)
	}, 10*time.Second, 250*time.Millisecond).Should(Succeed(), "Could not remove finalizers from DevWorkspace")

	if isNotFound {
		return
	}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, dwNN, dw)
		return err != nil && k8sErrors.IsNotFound(err)
	}, 10*time.Second, 250*time.Millisecond).Should(BeTrue(), "DevWorkspace not deleted after timeout")
}

func cleanupPVC(name string) {
	By("Cleaning up shared PVC")
	pvc := &corev1.PersistentVolumeClaim{}
	pvcNN := namespacedName(name, testNamespace)
	Eventually(func() error {
		err := k8sClient.Get(ctx, pvcNN, pvc)
		if k8sErrors.IsNotFound(err) {
			return nil
		}
		// Need to clear finalizers to allow PVC to be cleaned up
		pvc.Finalizers = nil
		err = k8sClient.Update(ctx, pvc)
		if err == nil {
			return err
		}
		return fmt.Errorf("PVC not deleted yet")
	}, timeout, interval).Should(Succeed(), "PVC should be deleted")
}

func createObject(obj crclient.Object) {
	Expect(k8sClient.Create(ctx, obj)).Should(Succeed())
	Eventually(func() error {
		return k8sClient.Get(ctx, namespacedName(obj.GetName(), obj.GetNamespace()), obj)
	}, 10*time.Second, 250*time.Millisecond).Should(Succeed(), "Creating %s with name %s", obj.GetObjectKind(), obj.GetName())
}

func deleteObject(obj crclient.Object) {
	Expect(k8sClient.Delete(ctx, obj)).Should(Succeed())
	Eventually(func() bool {
		err := k8sClient.Get(ctx, namespacedName(obj.GetName(), obj.GetNamespace()), obj)
		return k8sErrors.IsNotFound(err)
	}, 10*time.Second, 250*time.Millisecond).Should(BeTrue(), "Deleting %s with name %s", obj.GetObjectKind(), obj.GetName())
}

func markRoutingReady(mainUrl, routingName string) {
	namespacedName := namespacedName(routingName, testNamespace)
	routing := &controllerv1alpha1.DevWorkspaceRouting{}
	Eventually(func() error {
		err := k8sClient.Get(ctx, namespacedName, routing)
		if err != nil {
			return err
		}
		routing.Status.Phase = controllerv1alpha1.RoutingReady
		mainAttributes := controllerv1alpha1.Attributes{}
		mainAttributes.PutString("type", "main")
		exposedEndpoint := controllerv1alpha1.ExposedEndpoint{
			Name:       "test-endpoint",
			Url:        mainUrl,
			Attributes: mainAttributes,
		}
		routing.Status.ExposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{
			"test-endpoint": {
				exposedEndpoint,
			},
		}
		return k8sClient.Status().Update(ctx, routing)
	}, 30*time.Second, 250*time.Millisecond).Should(Succeed(), "Update DevWorkspaceRouting to have mainUrl and be ready")
}

func markDeploymentReady(deploymentName string) {
	namespacedName := namespacedName(deploymentName, testNamespace)
	deploy := &appsv1.Deployment{}
	Eventually(func() error {
		err := k8sClient.Get(ctx, namespacedName, deploy)
		if err != nil {
			return err
		}
		deploy.Status.ReadyReplicas = 1
		deploy.Status.Replicas = 1
		deploy.Status.AvailableReplicas = 1
		deploy.Status.UpdatedReplicas = 1
		return k8sClient.Status().Update(ctx, deploy)
	}, 30*time.Second, 250*time.Millisecond).Should(Succeed(), "Update Deployment to have 1 ready replica")
}

func scaleDeploymentToZero(deploymentName string) {
	namespacedName := namespacedName(deploymentName, testNamespace)
	deploy := &appsv1.Deployment{}
	Eventually(func() error {
		err := k8sClient.Get(ctx, namespacedName, deploy)
		if err != nil {
			return err
		}
		deploy.Status.ReadyReplicas = 0
		deploy.Status.Replicas = 0
		deploy.Status.AvailableReplicas = 0
		deploy.Status.UpdatedReplicas = 0
		return k8sClient.Status().Update(ctx, deploy)
	}, 30*time.Second, 250*time.Millisecond).Should(Succeed(), "Update Deployment to have 1 ready replica")
}

func devworkspaceOwnerRef(wksp *dw.DevWorkspace) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         "workspace.devfile.io/v1alpha2",
		Kind:               "DevWorkspace",
		Name:               wksp.Name,
		UID:                wksp.UID,
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}
}

func generateSecret(name string, secretType corev1.SecretType) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
			Annotations: map[string]string{},
		},
		Type: secretType,
		Data: map[string][]byte{},
	}
}

func generateConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				constants.DevWorkspaceWatchConfigMapLabel: "true",
			},
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}
}

func volumeFromSecret(secret *corev1.Secret) corev1.Volume {
	modeReadOnly := int32(0640)
	return corev1.Volume{
		Name: common.AutoMountSecretVolumeName(secret.Name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secret.Name,
				DefaultMode: &modeReadOnly,
			},
		},
	}
}

func volumeFromConfigMap(cm *corev1.ConfigMap) corev1.Volume {
	modeReadOnly := int32(0640)
	return corev1.Volume{
		Name: common.AutoMountConfigMapVolumeName(cm.Name),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
				DefaultMode:          &modeReadOnly,
			},
		},
	}
}

func volumeMountFromSecret(secret *corev1.Secret, mountPath, subPath string) corev1.VolumeMount {
	if subPath != "" {
		mountPath = path.Join(mountPath, subPath)
	}
	return corev1.VolumeMount{
		Name:      common.AutoMountSecretVolumeName(secret.Name),
		ReadOnly:  true,
		MountPath: mountPath,
		SubPath:   subPath,
	}
}

func volumeMountFromConfigMap(cm *corev1.ConfigMap, mountPath, subPath string) corev1.VolumeMount {
	if subPath != "" {
		mountPath = path.Join(mountPath, subPath)
	}
	return corev1.VolumeMount{
		Name:      common.AutoMountConfigMapVolumeName(cm.Name),
		ReadOnly:  true,
		MountPath: mountPath,
		SubPath:   subPath,
	}
}

func namespacedName(name, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}
