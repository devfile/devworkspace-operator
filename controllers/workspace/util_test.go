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

package controllers_test

import (
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// deleteDevWorkspace forces a DevWorkspace to be deleted by removing all finalizers
func deleteDevWorkspace(name string) {
	dwNN := types.NamespacedName{Name: name, Namespace: testNamespace}
	dw := &dw.DevWorkspace{}
	dw.Name = name
	dw.Namespace = testNamespace
	Expect(k8sClient.Delete(ctx, dw)).Should(Succeed())
	err := k8sClient.Get(ctx, dwNN, dw)
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
	}).Should(BeTrue(), "DevWorkspace not deleted after timeout")
}

func markRoutingReady(mainUrl, routingName string) {
	namespacedName := types.NamespacedName{
		Name:      routingName,
		Namespace: testNamespace,
	}
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
	namespacedName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: testNamespace,
	}
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

func devworkspaceOwnerRef(wksp *dw.DevWorkspace) metav1.OwnerReference {
	boolTrue := true
	return metav1.OwnerReference{
		APIVersion:         "workspace.devfile.io/v1alpha2",
		Kind:               "DevWorkspace",
		Name:               wksp.Name,
		UID:                wksp.UID,
		Controller:         &boolTrue,
		BlockOwnerDeletion: &boolTrue,
	}
}
