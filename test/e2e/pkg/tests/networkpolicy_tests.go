//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/config"
)

var _ = ginkgo.Describe("[Workspace NetworkPolicy Tests]", ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	const workspaceName = "network-policy-test"

	var originalConfig *controllerv1alpha1.OperatorConfiguration
	// workspaceId is read from the workspace's status once it is running; the NetworkPolicy
	// is named and selects pods by workspace ID rather than by workspace name.
	var workspaceId string

	getWorkspacePolicy := func() (*networkingv1.NetworkPolicy, error) {
		policy := &networkingv1.NetworkPolicy{}
		err := config.AdminK8sClient.ControllerRuntimeClient().Get(context.Background(), types.NamespacedName{
			Name:      common.NetworkPolicyName(workspaceId),
			Namespace: config.DevWorkspaceNamespace,
		}, policy)
		return policy, err
	}

	// setNetworkPolicyConfig updates the network policy section of the global DWOC. A nil
	// npConfig removes the section entirely, which leaves provisioning disabled.
	setNetworkPolicyConfig := func(npConfig *controllerv1alpha1.NetworkPolicyConfig) {
		ctx := context.Background()
		dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
		err := config.AdminK8sClient.ControllerRuntimeClient().Get(ctx, types.NamespacedName{
			Name:      "devworkspace-operator-config",
			Namespace: config.OperatorNamespace,
		}, dwoc)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get DWOC: %s", err))
		}
		if dwoc.Config == nil {
			dwoc.Config = &controllerv1alpha1.OperatorConfiguration{}
		}
		if dwoc.Config.Workspace == nil {
			dwoc.Config.Workspace = &controllerv1alpha1.WorkspaceConfig{}
		}
		dwoc.Config.Workspace.NetworkPolicy = npConfig
		if err := config.AdminK8sClient.ControllerRuntimeClient().Update(ctx, dwoc); err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to update DWOC network policy configuration: %s", err))
		}
	}

	// forceReconcile triggers a reconcile of the workspace. Changing the DWOC updates the
	// operator's resolved configuration but does not enqueue the workspaces it affects, so a
	// running workspace would otherwise only pick the new configuration up on its next
	// unrelated event.
	forceReconcile := func() {
		patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"force-update":"%d"}}}`, time.Now().UnixNano()))
		workspace := &dw.DevWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workspaceName,
				Namespace: config.DevWorkspaceNamespace,
			},
		}
		err := config.DevK8sClient.ControllerRuntimeClient().Patch(context.Background(), workspace, crclient.RawPatch(types.MergePatchType, patch))
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to trigger a reconcile of the workspace: %s", err))
		}
	}

	enabledConfig := func() *controllerv1alpha1.NetworkPolicyConfig {
		return &controllerv1alpha1.NetworkPolicyConfig{
			Enabled: pointer.Bool(true),
			Egress:  []networkingv1.NetworkPolicyEgressRule{{}},
		}
	}

	ginkgo.BeforeAll(func() {
		// Save original DWOC configuration to restore after tests
		ctx := context.Background()
		dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
		err := config.AdminK8sClient.ControllerRuntimeClient().Get(ctx, types.NamespacedName{
			Name:      "devworkspace-operator-config",
			Namespace: config.OperatorNamespace,
		}, dwoc)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get original DWOC: %s", err))
		}
		if dwoc.Config != nil {
			originalConfig = dwoc.Config.DeepCopy()
		}

		// The workspace is created while provisioning is disabled, so that the tests below
		// cover enabling the feature for a workspace that already exists.
		setNetworkPolicyConfig(nil)
	})

	ginkgo.AfterAll(func() {
		// Clean up workspace and wait for PVC to be fully deleted
		// This prevents PVC conflicts in subsequent tests, especially in CI environments
		_ = config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)

		// Restore original DWOC configuration to prevent config leaks between test runs
		ctx := context.Background()
		dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
		err := config.AdminK8sClient.ControllerRuntimeClient().Get(ctx, types.NamespacedName{
			Name:      "devworkspace-operator-config",
			Namespace: config.OperatorNamespace,
		}, dwoc)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to get current DWOC for restoration: %s", err))
		}
		dwoc.Config = originalConfig
		if err := config.AdminK8sClient.ControllerRuntimeClient().Update(ctx, dwoc); err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to restore original DWOC configuration: %s", err))
		}
	})

	ginkgo.It("Creates no NetworkPolicy while provisioning is disabled", func() {
		workspaceFile := filepath.Join(getProjectRoot(), "test", "resources", "network-policy-test-workspace.yaml")
		commandResult, err := config.DevK8sClient.OcApplyWorkspace(config.DevWorkspaceNamespace, workspaceFile)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Failed to create workspace: %s %s", err.Error(), commandResult))
			return
		}

		deploy, err := config.DevK8sClient.WaitDevWsStatus(workspaceName, config.DevWorkspaceNamespace, dw.DevWorkspaceStatusRunning)
		if !deploy {
			ginkgo.Fail(fmt.Sprintf("Workspace didn't start properly. Error: %s", err))
		}

		status, err := config.DevK8sClient.GetDevWsStatus(workspaceName, config.DevWorkspaceNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		workspaceId = status.DevWorkspaceId
		gomega.Expect(workspaceId).NotTo(gomega.BeEmpty(), "Running workspace should have a workspace ID")

		gomega.Consistently(func() bool {
			_, err := getWorkspacePolicy()
			return k8sErrors.IsNotFound(err)
		}, "15s", "5s").Should(gomega.BeTrue(), "No NetworkPolicy should be created while provisioning is disabled")
	})

	ginkgo.It("Creates a NetworkPolicy owned by an already running workspace when provisioning is enabled", func() {
		setNetworkPolicyConfig(enabledConfig())
		forceReconcile()

		gomega.Eventually(func() error {
			_, err := getWorkspacePolicy()
			return err
		}, "2m", "5s").Should(gomega.Succeed(), "NetworkPolicy should be created for the existing workspace")

		policy, err := getWorkspacePolicy()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(policy.Spec.PodSelector.MatchLabels).To(gomega.HaveKeyWithValue(constants.DevWorkspaceIDLabel, workspaceId),
			"Policy should select only the pods of its own workspace")
		gomega.Expect(policy.Spec.PolicyTypes).To(gomega.ContainElement(networkingv1.PolicyTypeEgress))

		// The ownerReference is what cleans the policy up; there is no finalizer on the
		// DevWorkspace for it.
		gomega.Expect(policy.OwnerReferences).To(gomega.HaveLen(1), "Policy should be owned by its workspace")
		ownerref := policy.OwnerReferences[0]
		gomega.Expect(ownerref.Kind).To(gomega.Equal("DevWorkspace"))
		gomega.Expect(ownerref.Name).To(gomega.Equal(workspaceName))
		gomega.Expect(pointer.BoolDeref(ownerref.Controller, false)).To(gomega.BeTrue())

		workspace := &dw.DevWorkspace{}
		err = config.DevK8sClient.ControllerRuntimeClient().Get(context.Background(), types.NamespacedName{
			Name:      workspaceName,
			Namespace: config.DevWorkspaceNamespace,
		}, workspace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(workspace.Finalizers).NotTo(gomega.ContainElement(gomega.ContainSubstring("networkpolicy")),
			"NetworkPolicy cleanup should not rely on a finalizer")
	})

	ginkgo.It("Removes the NetworkPolicy when provisioning is disabled", func() {
		setNetworkPolicyConfig(nil)
		forceReconcile()

		gomega.Eventually(func() bool {
			_, err := getWorkspacePolicy()
			return k8sErrors.IsNotFound(err)
		}, "2m", "5s").Should(gomega.BeTrue(), "NetworkPolicy should be removed when provisioning is disabled")
	})

	ginkgo.It("Garbage collects the NetworkPolicy when the workspace is deleted", func() {
		setNetworkPolicyConfig(enabledConfig())
		forceReconcile()

		gomega.Eventually(func() error {
			_, err := getWorkspacePolicy()
			return err
		}, "2m", "5s").Should(gomega.Succeed(), "NetworkPolicy should be created again once provisioning is re-enabled")

		err := config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to delete DevWorkspace")

		gomega.Eventually(func() bool {
			_, err := getWorkspacePolicy()
			return k8sErrors.IsNotFound(err)
		}, "2m", "5s").Should(gomega.BeTrue(), "NetworkPolicy should be garbage collected once its workspace is deleted")
	})
})
