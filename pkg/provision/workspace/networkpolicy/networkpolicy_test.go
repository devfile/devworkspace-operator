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

package networkpolicy

import (
	"context"
	"fmt"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

const testNamespace = "test-namespace"

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
}

func getTestClusterAPI(t *testing.T, initialObjects ...client.Object) sync.ClusterAPI {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initialObjects...).Build()
	return sync.ClusterAPI{
		Ctx:    context.Background(),
		Client: fakeClient,
		Scheme: scheme,
		Logger: testr.New(t),
	}
}

// getTestDevWorkspace returns a DevWorkspace whose resolved config carries the given
// network policy configuration. A nil npConfig leaves the section unset. The workspace ID
// deliberately differs from the workspace name, so that tests cannot pass by using one
// where the other is expected.
func getTestDevWorkspace(name string, npConfig *v1alpha1.NetworkPolicyConfig) *common.DevWorkspaceWithConfig {
	return &common.DevWorkspaceWithConfig{
		DevWorkspace: &dw.DevWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
				UID:       types.UID(fmt.Sprintf("uid-%s", name)),
			},
			Status: dw.DevWorkspaceStatus{
				DevWorkspaceId: fmt.Sprintf("workspace%s", name),
			},
		},
		Config: config.GetConfigForTesting(&v1alpha1.OperatorConfiguration{
			Workspace: &v1alpha1.WorkspaceConfig{
				NetworkPolicy: npConfig,
			},
		}),
	}
}

func enabledConfig() *v1alpha1.NetworkPolicyConfig {
	return &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Egress:  []networkingv1.NetworkPolicyEgressRule{{}},
	}
}

// getPolicy reads the NetworkPolicy belonging to a workspace from the cluster.
func getPolicy(testdw *common.DevWorkspaceWithConfig, api sync.ClusterAPI) (*networkingv1.NetworkPolicy, error) {
	actual := &networkingv1.NetworkPolicy{}
	err := api.Client.Get(api.Ctx, types.NamespacedName{
		Name:      common.NetworkPolicyName(testdw.Status.DevWorkspaceId),
		Namespace: testdw.Namespace,
	}, actual)
	return actual, err
}

func TestGeneratedPolicySelectsOnlyItsOwnWorkspacePods(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", enabledConfig())
	policy := generateNetworkPolicy(testdw, enabledConfig())
	assert.Equal(t, common.NetworkPolicyName(testdw.Status.DevWorkspaceId), policy.Name, "Policy should be named after the workspace it governs")
	assert.Equal(t, testNamespace, policy.Namespace, "Policy should be created in the workspace namespace")
	assert.Equal(t, map[string]string{
		constants.DevWorkspaceIDLabel:   testdw.Status.DevWorkspaceId,
		constants.DevWorkspaceNameLabel: testdw.Name,
	}, policy.Labels, "Policy should carry the labels every per-workspace object carries, so that it is watched by the controller cache")
	assert.Equal(t, metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.DevWorkspaceIDLabel: testdw.Status.DevWorkspaceId,
		},
	}, policy.Spec.PodSelector, "Policy should select only the pods of its own workspace")
}

func TestGeneratedPoliciesForDifferentWorkspacesDoNotCollide(t *testing.T) {
	first := getTestDevWorkspace("first-devworkspace", enabledConfig())
	second := getTestDevWorkspace("second-devworkspace", enabledConfig())
	firstPolicy := generateNetworkPolicy(first, enabledConfig())
	secondPolicy := generateNetworkPolicy(second, enabledConfig())
	assert.NotEqual(t, firstPolicy.Name, secondPolicy.Name,
		"Each workspace should get its own policy, otherwise two workspaces in a namespace would fight over one object")
	assert.NotEqual(t, firstPolicy.Spec.PodSelector, secondPolicy.Spec.PodSelector,
		"Each policy should select only its own workspace's pods")
}

// TestUnsetIngressDeniesAllIngress covers that both directions are always listed in
// policyTypes: a direction with no configured rules is denied, not left unrestricted.
func TestUnsetIngressDeniesAllIngress(t *testing.T) {
	tests := []struct {
		name    string
		ingress []networkingv1.NetworkPolicyIngressRule
	}{
		{name: "nil ingress list", ingress: nil},
		{name: "empty ingress list", ingress: []networkingv1.NetworkPolicyIngressRule{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			npConfig := &v1alpha1.NetworkPolicyConfig{
				Enabled: ptr.To(true),
				Ingress: tt.ingress,
				Egress:  []networkingv1.NetworkPolicyEgressRule{{}},
			}
			policy := generateNetworkPolicy(getTestDevWorkspace("test-devworkspace", npConfig), npConfig)
			assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				policy.Spec.PolicyTypes, "Both directions should always be in policyTypes, so that a direction without rules denies all traffic")
			assert.Nil(t, policy.Spec.Ingress,
				"A direction without rules must serialize as nil, since an empty slice is dropped by omitempty and would never match the cluster object")
		})
	}
}

func TestBothDirectionsNilDeniesAllTraffic(t *testing.T) {
	npConfig := &v1alpha1.NetworkPolicyConfig{Enabled: ptr.To(true)}
	policy := generateNetworkPolicy(getTestDevWorkspace("test-devworkspace", npConfig), npConfig)
	if !assert.NotNil(t, policy, "A policy should be built even when no rules are configured") {
		return
	}
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		policy.Spec.PolicyTypes, "With no rules configured both directions should be denied")
	assert.Nil(t, policy.Spec.Ingress, "No ingress rules should be produced")
	assert.Nil(t, policy.Spec.Egress, "No egress rules should be produced")
}

func TestPortProtocolIsDefaultedToTCP(t *testing.T) {
	port := intstr.FromInt(8080)
	npConfig := &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port}},
			},
		},
	}
	policy := generateNetworkPolicy(getTestDevWorkspace("test-devworkspace", npConfig), npConfig)
	actual := policy.Spec.Ingress[0].Ports[0].Protocol
	if !assert.NotNil(t, actual, "Protocol should be defaulted so the spec matches what the API server stores") {
		return
	}
	assert.Equal(t, corev1.ProtocolTCP, *actual, "Omitted protocol should default to TCP")
}

func TestExplicitPortProtocolIsPreserved(t *testing.T) {
	port := intstr.FromInt(53)
	udp := corev1.ProtocolUDP
	npConfig := &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Egress: []networkingv1.NetworkPolicyEgressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port, Protocol: &udp}},
			},
		},
	}
	policy := generateNetworkPolicy(getTestDevWorkspace("test-devworkspace", npConfig), npConfig)
	assert.Equal(t, corev1.ProtocolUDP, *policy.Spec.Egress[0].Ports[0].Protocol, "Explicit protocol should be preserved")
}

func TestGenerateDoesNotMutateConfig(t *testing.T) {
	port := intstr.FromInt(8080)
	npConfig := &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port}},
			},
		},
	}
	generateNetworkPolicy(getTestDevWorkspace("test-devworkspace", npConfig), npConfig)
	assert.Nil(t, npConfig.Ingress[0].Ports[0].Protocol,
		"Normalization must not write back into the shared resolved config")
}

func TestSyncCreatesPolicyWhenEnabled(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", enabledConfig())
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	retryErr := &dwerrors.RetryError{}
	if assert.Error(t, err, "Should return RetryError to indicate that the policy was created") {
		assert.ErrorAs(t, err, &retryErr, "Error should have RetryError type")
	}
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Should not return error once the policy is in sync")

	_, err = getPolicy(testdw, api)
	assert.NoError(t, err, "Policy should exist on the cluster")
}

// TestSyncOwnsPolicyByItsWorkspace covers the reason no finalizer is needed: the policy is
// garbage collected by the cluster along with the DevWorkspace that owns it.
func TestSyncOwnsPolicyByItsWorkspace(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", enabledConfig())
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was created")

	policy, err := getPolicy(testdw, api)
	if !assert.NoError(t, err, "Policy should exist on the cluster") {
		return
	}
	if !assert.Len(t, policy.OwnerReferences, 1, "Policy should be owned by the workspace it governs, so that it is garbage collected with it") {
		return
	}
	ownerref := policy.OwnerReferences[0]
	assert.Equal(t, testdw.Name, ownerref.Name, "Policy should be owned by its own workspace")
	assert.Equal(t, testdw.UID, ownerref.UID, "Policy should be owned by its own workspace")
	assert.Equal(t, "DevWorkspace", ownerref.Kind, "Policy should be owned by its own workspace")
	assert.True(t, ptr.Deref(ownerref.Controller, false), "Workspace should be the controller of its policy")
}

// TestSyncTreatsApiServerDefaultedProtocolAsInSync covers the round trip normalizePorts
// exists for: the configuration omits the port protocol, while the object stored on the
// cluster carries the protocol the API server defaulted. Without normalization the two
// never compare equal and every reconcile requests another update.
func TestSyncTreatsApiServerDefaultedProtocolAsInSync(t *testing.T) {
	port := intstr.FromInt(8080)
	tcp := corev1.ProtocolTCP
	npConfig := &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port}},
			},
		},
	}

	testdw := getTestDevWorkspace("test-devworkspace", npConfig)
	defaultedByApiServer := generateNetworkPolicy(testdw, &v1alpha1.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Port: &port, Protocol: &tcp}},
			},
		},
	})

	// The policy on the cluster was created by a previous sync, so it already carries the
	// ownerReference; without it the sync would request an update over the missing ownerref
	// rather than over the protocol under test.
	err := controllerutil.SetControllerReference(testdw.DevWorkspace, defaultedByApiServer, scheme)
	assert.NoError(t, err)

	api := getTestClusterAPI(t, testdw.DevWorkspace, defaultedByApiServer)
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err,
		"A policy stored with the protocol defaulted by the API server should be considered in sync, otherwise every reconcile requests an update")
}

func TestSyncDoesNothingWhenConfigSectionIsUnset(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", nil)
	testdw.Config.Workspace.NetworkPolicy = nil
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Should not return error when no network policy is configured")

	_, err = getPolicy(testdw, api)
	assert.True(t, k8sErrors.IsNotFound(err), "No policy should be created")
}

func TestSyncDeletesPolicyWhenDisabled(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", enabledConfig())
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was created")
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Policy should be in sync")

	testdw.Config.Workspace.NetworkPolicy.Enabled = ptr.To(false)
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Disabling should not return an error")

	_, err = getPolicy(testdw, api)
	assert.True(t, k8sErrors.IsNotFound(err), "Policy should be removed when the feature is disabled")
}

// TestSyncLeavesOtherWorkspacesPolicyAlone guards the per-workspace scope of the delete: a
// workspace that has the feature disabled must not remove the policy governing a different
// workspace in the same namespace.
func TestSyncLeavesOtherWorkspacesPolicyAlone(t *testing.T) {
	restricted := getTestDevWorkspace("restricted-devworkspace", enabledConfig())
	unrestricted := getTestDevWorkspace("unrestricted-devworkspace", nil)
	unrestricted.Config.Workspace.NetworkPolicy = nil
	api := getTestClusterAPI(t, restricted.DevWorkspace, unrestricted.DevWorkspace)
	err := SyncNetworkPolicy(restricted, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was created")

	err = SyncNetworkPolicy(unrestricted, api)
	assert.NoError(t, err, "Syncing a workspace with no policy configured should not return an error")

	_, err = getPolicy(restricted, api)
	assert.NoError(t, err, "A workspace must not delete the policy that governs another workspace")
}

func TestSyncCreatesDenyAllPolicyWhenNoRulesAreConfigured(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", &v1alpha1.NetworkPolicyConfig{Enabled: ptr.To(true)})
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was created")
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Should not return error once the policy is in sync")

	policy, err := getPolicy(testdw, api)
	if !assert.NoError(t, err, "A deny-all policy should be created when the feature is enabled without rules") {
		return
	}
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		policy.Spec.PolicyTypes, "Both directions should be denied when no rules are configured")
}

// TestSyncUpdatesPolicyWhenAllRulesAreCleared covers that clearing the rules tightens the
// existing policy into a deny-all one rather than leaving the previous rules on the cluster.
func TestSyncUpdatesPolicyWhenAllRulesAreCleared(t *testing.T) {
	testdw := getTestDevWorkspace("test-devworkspace", enabledConfig())
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was created")
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Policy should be in sync")

	testdw.Config.Workspace.NetworkPolicy.Egress = nil
	testdw.Config.Workspace.NetworkPolicy.Ingress = nil
	err = SyncNetworkPolicy(testdw, api)
	assert.Error(t, err, "Should return RetryError to indicate that the policy was updated")
	err = SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Policy should be in sync again")

	policy, err := getPolicy(testdw, api)
	if !assert.NoError(t, err, "The policy should still exist") {
		return
	}
	assert.Nil(t, policy.Spec.Egress, "The egress rules removed from the configuration should be removed from the cluster object")
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		policy.Spec.PolicyTypes, "Both directions should be denied once all rules are cleared")
}

func TestSyncIsTolerantOfMissingPolicyWhenDisabled(t *testing.T) {
	npConfig := enabledConfig()
	npConfig.Enabled = ptr.To(false)
	testdw := getTestDevWorkspace("test-devworkspace", npConfig)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err, "Deleting an absent policy should not return an error")
}
