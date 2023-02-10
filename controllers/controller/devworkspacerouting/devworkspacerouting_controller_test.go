// Copyright (c) 2019-2023 Red Hat, Inc.
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

package devworkspacerouting_test

import (
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DevWorkspaceRouting Controller", func() {
	Context("Basic DevWorkspaceRouting Tests", func() {
		It("Gets Preparing status", func() {
			By("Creating a new DevWorkspaceRouting object")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			createdDWR := createPreparingDWR(testWorkspaceID, devWorkspaceRoutingName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should exist in cluster")

			By("Checking DevWorkspaceRouting Status is updated to preparing")
			Eventually(func() (phase controllerv1alpha1.DevWorkspaceRoutingPhase, err error) {
				if err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR); err != nil {
					return "", err
				}
				return createdDWR.Status.Phase, nil
			}, timeout, interval).Should(Equal(controllerv1alpha1.RoutingPreparing), "DevWorkspaceRouting should have Preparing phase")

			Expect(createdDWR.Status.Message).ShouldNot(BeNil(), "Status message should be set for DevWorkspaceRoutings in Preparing phase")

			deleteDevWorkspaceRouting(devWorkspaceRoutingName)
			// Services and Ingresses aren't created since the DWR was stuck in preparing phase, no need to clean them up
		})

		It("Gets Ready Status on OpenShift", func() {
			By("Setting infrastructure to OpenShift")
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			By("Creating a new DevWorkspaceRouting object")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			createdDWR := createDWR(testWorkspaceID, devWorkspaceRoutingName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should exist in cluster")

			By("Checking DevWorkspaceRouting Status is updated to Ready")
			Eventually(func() (phase controllerv1alpha1.DevWorkspaceRoutingPhase, err error) {
				if err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR); err != nil {
					return "", err
				}
				return createdDWR.Status.Phase, nil
			}, timeout, interval).Should(Equal(controllerv1alpha1.RoutingReady), "DevWorkspaceRouting should have Ready phase")

			Expect(createdDWR.Status.Message).ShouldNot(BeNil(), "Status message should be set for preparing DevWorkspaceRoutings")
			Expect(createdDWR.Status.Message).Should(Equal("DevWorkspaceRouting prepared"), "Status message should indicate that the DevWorkspaceRouting is prepared")

			deleteDevWorkspaceRouting(devWorkspaceRoutingName)
			deleteService(common.ServiceName(testWorkspaceID), testNamespace)
			deleteService(common.EndpointName(discoverableEndpointName), testNamespace)
			deleteRoute(exposedEndPointName, testNamespace)
			deleteRoute(discoverableEndpointName, testNamespace)
		})

		It("Gets Ready Status on Kubernetes", func() {
			By("Setting infrastructure to Kubernetes")
			infrastructure.InitializeForTesting(infrastructure.Kubernetes)

			By("Creating a new DevWorkspaceRouting object")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			createdDWR := createDWR(testWorkspaceID, devWorkspaceRoutingName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should exist in cluster")

			By("Checking DevWorkspaceRouting Status is updated to Ready")
			Eventually(func() (phase controllerv1alpha1.DevWorkspaceRoutingPhase, err error) {
				if err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR); err != nil {
					return "", err
				}
				return createdDWR.Status.Phase, nil
			}, timeout, interval).Should(Equal(controllerv1alpha1.RoutingReady), "DevWorkspaceRouting should have Ready phase")
			Expect(createdDWR.Status.Message).ShouldNot(BeNil(), "Status message should be set for preparing DevWorkspaceRoutings")
			Expect(createdDWR.Status.Message).Should(Equal("DevWorkspaceRouting prepared"), "Status message should indicate that the DevWorkspaceRouting is prepared")

			deleteDevWorkspaceRouting(devWorkspaceRoutingName)
			deleteService(common.ServiceName(testWorkspaceID), testNamespace)
			deleteService(common.EndpointName(discoverableEndpointName), testNamespace)
			deleteIngress(exposedEndPointName, testNamespace)
			deleteIngress(discoverableEndpointName, testNamespace)
		})
	})
})
