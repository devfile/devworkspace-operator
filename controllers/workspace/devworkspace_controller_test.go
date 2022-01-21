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
	"fmt"
	"os"
	"path/filepath"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func loadObjectFromFile(objName string, obj client.Object, filename string) error {
	path := filepath.Join("testdata", filename)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, obj)
	if err != nil {
		return err
	}
	obj.SetNamespace(testNamespace)
	obj.SetName(objName)

	return nil
}

var _ = Describe("DevWorkspace Controller", func() {
	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

	Context("Basic DevWorkspace Tests", func() {
		It("Sets DevWorkspace ID and Starting status", func() {
			By("Reading DevWorkspace from testdata file")
			devworkspace := &dw.DevWorkspace{}
			err := loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := types.NamespacedName{
				Namespace: testNamespace,
				Name:      devWorkspaceName,
			}
			defer deleteDevWorkspace(devWorkspaceName)

			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwNamespacedName, createdDW)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspace should exist in cluster")

			By("Checking DevWorkspace ID has been set")
			Eventually(func() (devworkspaceID string, err error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.DevWorkspaceId, nil
			}, timeout, interval).Should(Not(Equal("")), "Should set DevWorkspace ID after creation")

			By("Checking DevWorkspace Status is updated to starting")
			Eventually(func() (phase dw.DevWorkspacePhase, err error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusStarting), "DevWorkspace should have Starting phase")
			Expect(createdDW.Status.Message).ShouldNot(BeEmpty(), "Status message should be set for starting workspaces")
			startingCondition := conditions.GetConditionByType(createdDW.Status.Conditions, conditions.Started)
			Expect(startingCondition).ShouldNot(BeNil(), "Should have 'Starting' condition")
			Expect(startingCondition.Status).Should(Equal(corev1.ConditionTrue), "Starting condition should be 'true'")
		})
	})

	Context("Workspace Objects creation", func() {

		BeforeEach(func() {
			By("Reading DevWorkspace from testdata file")
			devworkspace := &dw.DevWorkspace{}
			err := loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, createdDW); err != nil {
					return false
				}
				return createdDW.Status.DevWorkspaceId != ""
			}, timeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
		})

		It("Creates roles and rolebindings", func() {
			By("Checking that common role is created")
			dwRole := &rbacv1.Role{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: common.WorkspaceRoleName(), Namespace: testNamespace}, dwRole); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common Role should be created for DevWorkspace")

			By("Checking that common rolebinding is created")
			dwRoleBinding := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: common.WorkspaceRolebindingName(), Namespace: testNamespace}, dwRoleBinding); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common RoleBinding should be created for DevWorkspace")
			Expect(dwRoleBinding.RoleRef.Name).Should(Equal(dwRole.Name), "Rolebinding should refer to DevWorkspace role")
			expectedSubject := rbacv1.Subject{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     fmt.Sprintf("system:serviceaccounts:%s", testNamespace),
			}
			Expect(dwRoleBinding.Subjects).Should(ContainElement(expectedSubject), "Rolebinding should bind to serviceaccounts in current namespace")
		})

		It("Creates DevWorkspaceRouting", func() {
			By("Getting existing DevWorkspace from cluster")
			devworkspace := &dw.DevWorkspace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace)).Should(Succeed())
			workspaceID := devworkspace.Status.DevWorkspaceId
			Expect(workspaceID).ShouldNot(BeEmpty(), "DevWorkspaceID not set")

			By("Checking that DevWorkspaceRouting is created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: dwrName, Namespace: testNamespace}, dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting is present on cluster")

			Expect(string(dwr.Spec.RoutingClass)).Should(Equal(devworkspace.Spec.RoutingClass), "RoutingClass should be propagated to DevWorkspaceRouting")
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(dwr.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Routing should be owned by DevWorkspace")
			Expect(dwr.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs Routing mainURL to DevWorkspace", func() {
			By("Getting existing DevWorkspace from cluster")
			devworkspace := &dw.DevWorkspace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace)).Should(Succeed())
			workspaceID := devworkspace.Status.DevWorkspaceId
			Expect(workspaceID).ShouldNot(BeEmpty(), "DevWorkspaceID not set")

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			Eventually(func() (string, error) {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace); err != nil {
					return "", err
				}
				return devworkspace.Status.MainUrl, nil
			}, timeout, interval).Should(Equal("test-url"))

		})

		It("Creates workspace metadata configmap", func() {
			By("Getting existing DevWorkspace from cluster")
			devworkspace := &dw.DevWorkspace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace)).Should(Succeed())
			workspaceID := devworkspace.Status.DevWorkspaceId
			Expect(workspaceID).ShouldNot(BeEmpty(), "DevWorkspaceID not set")

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			metadataCM := &corev1.ConfigMap{}
			Eventually(func() error {
				cmNN := types.NamespacedName{
					Name:      common.MetadataConfigMapName(workspaceID),
					Namespace: testNamespace,
				}
				return k8sClient.Get(ctx, cmNN, metadataCM)
			}, timeout, interval).Should(Succeed(), "Should create workspace metadata configmap")

			// Check that metadata configmap is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(metadataCM.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Metadata configmap should be owned by DevWorkspace")
			Expect(metadataCM.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")

			originalDevfileYaml, originalPresent := metadataCM.Data["original.devworkspace.yaml"]
			Expect(originalPresent).Should(BeTrue(), "Metadata configmap should contain original devfile spec")
			originalDevfile := &dw.DevWorkspaceTemplateSpec{}
			Expect(yaml.Unmarshal([]byte(originalDevfileYaml), originalDevfile)).Should(Succeed())
			Expect(originalDevfile).Should(Equal(&devworkspace.Spec.Template))
			_, flattenedPresent := metadataCM.Data["flattened.devworkspace.yaml"]
			Expect(flattenedPresent).Should(BeTrue(), "Metadata configmap should contain flattened devfile spec")
		})

		It("Syncs the DevWorkspace ServiceAccount", func() {
			By("Getting existing DevWorkspace from cluster")
			devworkspace := &dw.DevWorkspace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace)).Should(Succeed())
			workspaceID := devworkspace.Status.DevWorkspaceId
			Expect(workspaceID).ShouldNot(BeEmpty(), "DevWorkspaceID not set")

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				saNN := types.NamespacedName{
					Name:      common.ServiceAccountName(workspaceID),
					Namespace: testNamespace,
				}
				return k8sClient.Get(ctx, saNN, sa)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace ServiceAccount")

			// Check that SA is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(sa.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace ServiceAccount should be owned by DevWorkspace")
			Expect(sa.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs DevWorkspace Deployment", func() {
			By("Getting existing DevWorkspace from cluster")
			devworkspace := &dw.DevWorkspace{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace)).Should(Succeed())
			workspaceID := devworkspace.Status.DevWorkspaceId
			Expect(workspaceID).ShouldNot(BeEmpty(), "DevWorkspaceID not set")

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			Eventually(func() error {
				deployNN := types.NamespacedName{
					Name:      common.DeploymentName(workspaceID),
					Namespace: testNamespace,
				}
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace Deployment")

			// Check that Deployment is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(deploy.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace Deployment should be owned by DevWorkspace")
			Expect(deploy.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

	})
})
