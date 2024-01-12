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

package devworkspacerouting_test

import (
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("DevWorkspaceRouting Controller", func() {
	Context("Basic DevWorkspaceRouting Tests", func() {
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

		Context("Kubernetes - DevWorkspaceRouting Objects creation", func() {
			BeforeEach(func() {
				infrastructure.InitializeForTesting(infrastructure.Kubernetes)
				createDWR(testWorkspaceID, devWorkspaceRoutingName)
			})

			AfterEach(func() {
				deleteDevWorkspaceRouting(devWorkspaceRoutingName)
				deleteService(common.ServiceName(testWorkspaceID), testNamespace)
				deleteService(common.EndpointName(discoverableEndpointName), testNamespace)
				deleteIngress(exposedEndPointName, testNamespace)
				deleteIngress(discoverableEndpointName, testNamespace)
			})
			It("Creates services", func() {
				createdDWR := getExistingDevWorkspaceRouting(devWorkspaceRoutingName)

				By("Checking single service is created for all exposed endpoints ")
				createdService := &corev1.Service{}
				serviceNamespacedName := namespacedName(common.ServiceName(testWorkspaceID), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, serviceNamespacedName, createdService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service should exist in cluster")
				Expect(createdService.Spec.Selector).Should(Equal(createdDWR.Spec.PodSelector), "Service should have pod selector from DevWorkspace metadata")
				Expect(createdService.Labels).Should(Equal(ExpectedLabels), "Service should contain DevWorkspace ID label")
				expectedOwnerReference := devWorkspaceRoutingOwnerRef(createdDWR)
				Expect(createdService.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Service should be owned by DevWorkspaceRouting")

				By("Checking service has expected ports")
				var expectedServicePorts []corev1.ServicePort
				expectedServicePorts = append(expectedServicePorts, corev1.ServicePort{
					Name:       common.EndpointName(exposedEndPointName),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(exposedTargetPort),
					TargetPort: intstr.FromInt(exposedTargetPort),
				})

				expectedServicePorts = append(expectedServicePorts, corev1.ServicePort{
					Name:       common.EndpointName(discoverableEndpointName),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(discoverableTargetPort),
					TargetPort: intstr.FromInt(discoverableTargetPort),
				})

				Expect(len(createdService.Spec.Ports)).Should(Equal(2), fmt.Sprintf("Only two ports should be exposed: %s and %s. The remaining endpoint in the DevWorkspaceRouting spec has None exposure.", exposedEndPointName, discoverableEndpointName))
				Expect(createdService.Spec.Ports).Should(Equal(expectedServicePorts), "Service should contain expected ports")

				By("Checking service is created for discoverable endpoint")
				discoverableEndpointService := &corev1.Service{}
				discoverableEndpointServiceNamespacedName := namespacedName(common.EndpointName(discoverableEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, discoverableEndpointServiceNamespacedName, discoverableEndpointService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service for discoverable endpoint should exist in cluster")
				Expect(len(discoverableEndpointService.Spec.Ports)).Should(Equal(1), "Service for discoverable endpoint should only have a single port")
				Expect(discoverableEndpointService.Spec.Ports[0].Port).Should(Equal(int32(discoverableTargetPort)))
				Expect(discoverableEndpointService.Spec.Ports[0].TargetPort).Should(Equal(intstr.FromInt(discoverableTargetPort)))
				Expect(discoverableEndpointService.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceDiscoverableServiceAnnotation, "true"), "Service type should have discoverable service annotation")

				By("Checking service is not created for non-exposed endpoint")
				nonExposedService := &corev1.Service{}
				nonExposedServiceNamespacedName := namespacedName(common.RouteName(testWorkspaceID, nonExposedEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, nonExposedServiceNamespacedName, nonExposedService)
					return k8sErrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Route for non-exposed endpoint should not exist in cluster")
			})

			It("Creates ingress", func() {
				createdDWR := getExistingDevWorkspaceRouting(devWorkspaceRoutingName)

				By("Checking ingress is created")
				createdIngress := networkingv1.Ingress{}
				ingressNamespacedName := namespacedName(common.RouteName(testWorkspaceID, exposedEndPointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, ingressNamespacedName, &createdIngress)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Ingress should exist in cluster")

				Expect(createdIngress.Labels).Should(Equal(ExpectedLabels), "Ingress should contain DevWorkspace ID label")
				expectedOwnerReference := devWorkspaceRoutingOwnerRef(createdDWR)
				Expect(createdIngress.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Ingress should be owned by DevWorkspaceRouting")
				Expect(createdIngress.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceEndpointNameAnnotation, exposedEndPointName), "Ingress should have endpoint name annotation")

				By("Checking ingress points to service")
				createdService := &corev1.Service{}
				serviceNamespacedName := namespacedName(common.ServiceName(testWorkspaceID), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, serviceNamespacedName, createdService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service should exist in cluster")

				var targetPorts []intstr.IntOrString
				var ports []int32
				for _, servicePort := range createdService.Spec.Ports {
					targetPorts = append(targetPorts, servicePort.TargetPort)
					ports = append(ports, servicePort.Port)
				}
				Expect(len(createdIngress.Spec.Rules)).Should(Equal(1), "Expected only a single rule for the ingress")
				ingressRule := createdIngress.Spec.Rules[0]
				Expect(ingressRule.HTTP.Paths[0].Backend.Service.Name).Should(Equal(createdService.Name), "Ingress backend service name should be service name")
				Expect(ports).Should(ContainElement(ingressRule.HTTP.Paths[0].Backend.Service.Port.Number), "Ingress backend service port should be in service ports")
				Expect(targetPorts).Should(ContainElement(intstr.FromInt(int(ingressRule.HTTP.Paths[0].Backend.Service.Port.Number))), "Ingress backend service port should be service target ports")

				By("Checking ingress is created for discoverable endpoint")
				discoverableEndpointIngress := networkingv1.Ingress{}
				discoverableEndpointIngressNN := namespacedName(common.RouteName(testWorkspaceID, discoverableEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, discoverableEndpointIngressNN, &discoverableEndpointIngress)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Ingress should exist in cluster")
				Expect(discoverableEndpointIngress.Labels).Should(Equal(ExpectedLabels), "Ingress should contain DevWorkspace ID label")
				Expect(discoverableEndpointIngress.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Ingress should be owned by DevWorkspaceRouting")
				Expect(discoverableEndpointIngress.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceEndpointNameAnnotation, discoverableEndpointName), "Ingress should have endpoint name annotation")

				By("Checking ingress for discoverable endpoint points to service")
				Expect(len(discoverableEndpointIngress.Spec.Rules)).Should(Equal(1), "Expected only a single rule for the ingress")
				ingressRule = createdIngress.Spec.Rules[0]
				Expect(ingressRule.HTTP.Paths[0].Backend.Service.Name).Should(Equal(createdService.Name), "Ingress backend service name should be service name")
				Expect(ports).Should(ContainElement(ingressRule.HTTP.Paths[0].Backend.Service.Port.Number), "Ingress backend service port should be in service ports")
				Expect(targetPorts).Should(ContainElement(intstr.FromInt(int(ingressRule.HTTP.Paths[0].Backend.Service.Port.Number))), "Ingress backend service port should be service target ports")

				By("Checking ingress is not created for non-exposed endpoint")
				nonExposedIngress := networkingv1.Ingress{}
				nonExposedIngressNamespacedName := namespacedName(common.RouteName(testWorkspaceID, nonExposedEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, nonExposedIngressNamespacedName, &nonExposedIngress)
					return k8sErrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Ingress for non-exposed endpoint should not exist in cluster")
			})

		})

		Context("OpenShift - DevWorkspaceRouting Objects creation", func() {

			BeforeEach(func() {
				infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
				createDWR(testWorkspaceID, devWorkspaceRoutingName)
			})

			AfterEach(func() {
				deleteDevWorkspaceRouting(devWorkspaceRoutingName)
				deleteService(common.ServiceName(testWorkspaceID), testNamespace)
				deleteService(common.EndpointName(discoverableEndpointName), testNamespace)
				deleteRoute(exposedEndPointName, testNamespace)
				deleteRoute(discoverableEndpointName, testNamespace)
			})
			It("Creates services", func() {
				createdDWR := getExistingDevWorkspaceRouting(devWorkspaceRoutingName)

				By("Checking single service for all exposed endpoints is created")
				createdService := &corev1.Service{}
				serviceNamespacedName := namespacedName(common.ServiceName(testWorkspaceID), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, serviceNamespacedName, createdService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service should exist in cluster")
				Expect(createdService.Spec.Selector).Should(Equal(createdDWR.Spec.PodSelector), "Service should have pod selector from DevWorkspaceRouting")
				Expect(createdService.Labels).Should(Equal(ExpectedLabels), "Service should contain DevWorkspace ID label")
				expectedOwnerReference := devWorkspaceRoutingOwnerRef(createdDWR)
				Expect(createdService.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Service should be owned by DevWorkspaceRouting")

				By("Checking service has expected ports")
				var expectedServicePorts []corev1.ServicePort
				expectedServicePorts = append(expectedServicePorts, corev1.ServicePort{
					Name:       common.EndpointName(exposedEndPointName),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(exposedTargetPort),
					TargetPort: intstr.FromInt(exposedTargetPort),
				})

				expectedServicePorts = append(expectedServicePorts, corev1.ServicePort{
					Name:       common.EndpointName(discoverableEndpointName),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(discoverableTargetPort),
					TargetPort: intstr.FromInt(discoverableTargetPort),
				})
				Expect(createdService.Spec.Ports).Should(Equal(expectedServicePorts), "Service should contain expected ports")

				By("Checking service is created for discoverable endpoint")
				discoverableEndpointService := &corev1.Service{}
				discoverableEndpointServiceNamespacedName := namespacedName(common.EndpointName(discoverableEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, discoverableEndpointServiceNamespacedName, discoverableEndpointService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service for discoverable endpoint should exist in cluster")
				Expect(len(discoverableEndpointService.Spec.Ports)).Should(Equal(1), "Service for discoverable endpoint should only have a single port")
				Expect(discoverableEndpointService.Spec.Ports[0].Port).Should(Equal(int32(discoverableTargetPort)))
				Expect(discoverableEndpointService.Spec.Ports[0].TargetPort).Should(Equal(intstr.FromInt(discoverableTargetPort)))
				Expect(discoverableEndpointService.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceDiscoverableServiceAnnotation, "true"), "Service type should have discoverable service annotation")

				By("Checking service is not created for non-exposed endpoint")
				nonExposedService := &corev1.Service{}
				nonExposedServiceNamespacedName := namespacedName(common.RouteName(testWorkspaceID, nonExposedEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, nonExposedServiceNamespacedName, nonExposedService)
					return k8sErrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Route for non-exposed endpoint should not exist in cluster")
			})

			It("Creates route", func() {
				createdDWR := getExistingDevWorkspaceRouting(devWorkspaceRoutingName)

				By("Checking route is created")
				createdRoute := routeV1.Route{}
				routeNamespacedName := namespacedName(common.RouteName(testWorkspaceID, exposedEndPointName), testNamespace)
				Eventually(func() error {
					err := k8sClient.Get(ctx, routeNamespacedName, &createdRoute)
					return err
				}, timeout, interval).Should(BeNil(), "Route should exist in cluster")

				Expect(createdRoute.Labels).Should(Equal(ExpectedLabels), "Route should contain DevWorkspace ID label")
				expectedOwnerReference := devWorkspaceRoutingOwnerRef(createdDWR)
				Expect(createdRoute.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Route should be owned by DevWorkspaceRouting")
				Expect(createdRoute.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceEndpointNameAnnotation, exposedEndPointName), "Route should have endpoint name annotation")

				By("Checking route points to service")
				createdService := &corev1.Service{}
				serviceNamespacedName := namespacedName(common.ServiceName(testWorkspaceID), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, serviceNamespacedName, createdService)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "Service should exist in cluster")

				var targetPorts []intstr.IntOrString
				var ports []int32
				for _, servicePort := range createdService.Spec.Ports {
					targetPorts = append(targetPorts, servicePort.TargetPort)
					ports = append(ports, servicePort.Port)
				}
				Expect(targetPorts).Should(ContainElement(createdRoute.Spec.Port.TargetPort), "Route target port should be in service target ports")
				Expect(ports).Should(ContainElement(createdRoute.Spec.Port.TargetPort.IntVal), "Route target port should be in service ports")
				Expect(createdRoute.Spec.To.Name).Should(Equal(createdService.Name), "Route target reference should be service name")

				By("Checking route is created for discoverable endpoint")
				discoverableRoute := routeV1.Route{}
				dsicoverableRouteNamespacedName := namespacedName(common.RouteName(testWorkspaceID, discoverableEndpointName), testNamespace)
				Eventually(func() error {
					err := k8sClient.Get(ctx, dsicoverableRouteNamespacedName, &discoverableRoute)
					return err
				}, timeout, interval).Should(BeNil(), "Route should exist in cluster")
				Expect(discoverableRoute.Labels).Should(Equal(ExpectedLabels), "Route should contain DevWorkspace ID label")
				Expect(discoverableRoute.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Route should be owned by DevWorkspaceRouting")
				Expect(discoverableRoute.ObjectMeta.Annotations).Should(HaveKeyWithValue(constants.DevWorkspaceEndpointNameAnnotation, discoverableEndpointName), "Route should have endpoint name annotation")

				By("Checking route for discoverable endpoint points to service")
				Expect(targetPorts).Should(ContainElement(discoverableRoute.Spec.Port.TargetPort), "Route target port should be in service target ports")
				Expect(ports).Should(ContainElement(discoverableRoute.Spec.Port.TargetPort.IntVal), "Route target port should be in service ports")
				Expect(discoverableRoute.Spec.To.Name).Should(Equal(createdService.Name), "Route target reference should be service name")

				By("Checking route is not created for non-exposed endpoint")
				nonExposedRoute := routeV1.Route{}
				nonExposedRouteNamespacedName := namespacedName(common.RouteName(testWorkspaceID, nonExposedEndpointName), testNamespace)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, nonExposedRouteNamespacedName, &nonExposedRoute)
					return k8sErrors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Route for non-exposed endpoint should not exist in cluster")
			})
		})
	})

	Context("Failure cases", func() {
		BeforeEach(func() {
			config.SetGlobalConfigForTesting(testControllerCfg)
			infrastructure.InitializeForTesting(infrastructure.Kubernetes)
			createDWR(testWorkspaceID, devWorkspaceRoutingName)
			getReadyDevWorkspaceRouting(testWorkspaceID)
		})

		AfterEach(func() {
			config.SetGlobalConfigForTesting(testControllerCfg)
			deleteDevWorkspaceRouting(devWorkspaceRoutingName)
			deleteService(common.ServiceName(testWorkspaceID), testNamespace)
			deleteService(common.EndpointName(discoverableEndpointName), testNamespace)
			deleteIngress(exposedEndPointName, testNamespace)
			deleteIngress(discoverableEndpointName, testNamespace)
		})
		It("Fails DevWorkspaceRouting with no routing class", func() {
			By("Creating DevWorkspaceRouting with no routing class")
			mainAttributes := controllerv1alpha1.Attributes{}
			mainAttributes.PutString("type", "main")
			machineEndpointsMap := map[string]controllerv1alpha1.EndpointList{
				testMachineName: {
					controllerv1alpha1.Endpoint{
						Name:       exposedEndPointName,
						Exposure:   controllerv1alpha1.PublicEndpointExposure,
						Attributes: mainAttributes,
						TargetPort: exposedTargetPort,
					},
				},
			}

			dwr := &controllerv1alpha1.DevWorkspaceRouting{
				Spec: controllerv1alpha1.DevWorkspaceRoutingSpec{
					DevWorkspaceId: testWorkspaceID,
					RoutingClass:   "",
					Endpoints:      machineEndpointsMap,
					PodSelector: map[string]string{
						constants.DevWorkspaceIDLabel: testWorkspaceID,
					},
				},
			}
			// Choose a unique name because a DWR will have already been created in the BeforeEach,
			// causing a conflict if we try reusing the same name
			dwrName := "dwr-with-no-routing-class"
			dwr.SetName(dwrName)
			dwr.SetNamespace(testNamespace)
			Expect(k8sClient.Create(ctx, dwr)).Should(Succeed())

			By("Checking that the DevWorkspaceRouting's has the failed status")
			dwrNamespacedName := namespacedName(dwrName, testNamespace)
			createdDWR := &controllerv1alpha1.DevWorkspaceRouting{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, dwrNamespacedName, createdDWR)
				if err != nil {
					return false, err
				}
				return createdDWR.Status.Phase == controllerv1alpha1.RoutingFailed, nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should be in failed phase")

			Expect(createdDWR.Status.Message).Should(Equal("DevWorkspaceRouting requires field routingClass to be set"),
				"DevWorkspaceRouting status message should indicate that the routingClass must be set")

			// Manual cleanup since we specified a DWR name that is different than what is specified in the AfterEach
			// No ingresses or services owned by the DWR-with-no-routing-class will be created, as it entered the failed phase early
			deleteDevWorkspaceRouting(dwrName)
		})

		It("Fails DevWorkspaceRouting when routing class removed", func() {
			By("Removing DevWorkspaceRouting's routing class")
			Eventually(func() error {
				createdDWR := getReadyDevWorkspaceRouting(devWorkspaceRoutingName)
				createdDWR.Spec.RoutingClass = ""
				return k8sClient.Update(ctx, createdDWR)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting routing class should be updated on cluster")

			By("Checking that the DevWorkspaceRouting's has the failed status")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			updatedDWR := &controllerv1alpha1.DevWorkspaceRouting{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, dwrNamespacedName, updatedDWR)
				if err != nil {
					return false, err
				}
				return updatedDWR.Status.Phase == controllerv1alpha1.RoutingFailed, nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should be in failed phase")

			Expect(updatedDWR.Status.Message).Should(Equal("DevWorkspaceRouting requires field routingClass to be set"),
				"DevWorkspaceRouting status message should indicate that the routingClass must be set")

		})

		It("Fails DevWorkspaceRouting with cluster-tls routing class on Kubernetes", func() {
			By("Setting cluster-tls DevWorkspaceRouting's routing class")
			Eventually(func() error {
				createdDWR := getReadyDevWorkspaceRouting(devWorkspaceRoutingName)
				createdDWR.Spec.RoutingClass = "cluster-tls"
				return k8sClient.Update(ctx, createdDWR)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting routing class should be updated on cluster")

			By("Checking that the DevWorkspaceRouting's has the failed status")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			updatedDWR := &controllerv1alpha1.DevWorkspaceRouting{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, dwrNamespacedName, updatedDWR)
				if err != nil {
					return false, err
				}
				return updatedDWR.Status.Phase == controllerv1alpha1.RoutingFailed, nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspaceRouting should be in failed phase")
		})

		It("Fails DevWorkspaceRouting when cluster host suffix missing on Kubernetes", func() {
			By("Removing cluster host suffix from DevWorkspace Operator Configuration")
			dwoc := testControllerCfg.DeepCopy()
			dwoc.Routing = &controllerv1alpha1.RoutingConfig{
				ClusterHostSuffix: "",
			}
			config.SetGlobalConfigForTesting(dwoc)

			By("Triggering a reconcile")
			Eventually(func() error {
				createdDWR := getReadyDevWorkspaceRouting(devWorkspaceRoutingName)
				createdDWR.Annotations = make(map[string]string)
				createdDWR.Annotations["test"] = "test"
				return k8sClient.Update(ctx, createdDWR)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting annotations should be updated on cluster")

			By("Checking that the DevWorkspaceRouting's has the failed status")
			dwrNamespacedName := namespacedName(devWorkspaceRoutingName, testNamespace)
			updatedDWR := &controllerv1alpha1.DevWorkspaceRouting{}
			Eventually(func() (controllerv1alpha1.DevWorkspaceRoutingPhase, error) {
				err := k8sClient.Get(ctx, dwrNamespacedName, updatedDWR)
				if err != nil {
					return "", err
				}
				return updatedDWR.Status.Phase, nil
			}, timeout, interval).Should(Equal(controllerv1alpha1.RoutingFailed), "DevWorkspaceRouting should be in failed phase")
		})
	})
})
