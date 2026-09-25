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
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace        = "test-namespace"
	testDevworkspaceName = "test-devworkspace"
)

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

func getTestDevWorkspaceWithConfig(name string, npConfig *v1alpha1.NetworkPolicyConfig) *common.DevWorkspaceWithConfig {
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

func getEnabledNetworkPolicyConfig() *v1alpha1.NetworkPolicyConfig {
	return &v1alpha1.NetworkPolicyConfig{
		Enabled: new(true),
	}
}

func getNetworkPolicyFromCluster(testDevworkspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) (*networkingv1.NetworkPolicy, error) {
	actual := &networkingv1.NetworkPolicy{}
	err := api.Client.Get(
		api.Ctx,
		types.NamespacedName{
			Name:      common.NetworkPolicyName(testDevworkspace.Status.DevWorkspaceId),
			Namespace: testDevworkspace.Namespace,
		},
		actual)

	return actual, err
}

func TestGeneratedPolicySelectsOnlyItsOwnWorkspacePods(t *testing.T) {
	testDevWorkspace := getTestDevWorkspaceWithConfig(testDevworkspaceName, getEnabledNetworkPolicyConfig())
	policy := generateNetworkPolicy(testDevWorkspace, getEnabledNetworkPolicyConfig())

	assert.Equal(t, common.NetworkPolicyName(testDevWorkspace.Status.DevWorkspaceId), policy.Name)
	assert.Equal(t, testNamespace, policy.Namespace)
	assert.Equal(t, map[string]string{
		constants.DevWorkspaceIDLabel:   testDevWorkspace.Status.DevWorkspaceId,
		constants.DevWorkspaceNameLabel: testDevWorkspace.Name,
	}, policy.Labels)
	assert.Equal(t, metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.DevWorkspaceIDLabel: testDevWorkspace.Status.DevWorkspaceId,
		},
	}, policy.Spec.PodSelector)
}

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
				Enabled: new(true),
				Ingress: tt.ingress,
				Egress:  []networkingv1.NetworkPolicyEgressRule{{}},
			}
			policy := generateNetworkPolicy(getTestDevWorkspaceWithConfig(testDevworkspaceName, npConfig), npConfig)

			assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}, policy.Spec.PolicyTypes)
			assert.Nil(t, policy.Spec.Ingress)
		})
	}
}

func TestBothDirectionsNilDeniesAllTraffic(t *testing.T) {
	npConfig := &v1alpha1.NetworkPolicyConfig{Enabled: new(true)}
	policy := generateNetworkPolicy(getTestDevWorkspaceWithConfig(testDevworkspaceName, npConfig), npConfig)

	assert.NotNil(t, policy)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}, policy.Spec.PolicyTypes)
	assert.Nil(t, policy.Spec.Ingress)
	assert.Nil(t, policy.Spec.Egress)
}

func TestSyncCreatesPolicyWhenEnabled(t *testing.T) {
	testDevworkspace := getTestDevWorkspaceWithConfig(testDevworkspaceName, getEnabledNetworkPolicyConfig())
	api := getTestClusterAPI(t, testDevworkspace.DevWorkspace)

	err := SyncNetworkPolicy(testDevworkspace, api)
	retryErr := &dwerrors.RetryError{}
	assert.Error(t, err)
	assert.ErrorAs(t, err, &retryErr)

	err = SyncNetworkPolicy(testDevworkspace, api)
	assert.NoError(t, err)

	policy, err := getNetworkPolicyFromCluster(testDevworkspace, api)
	assert.NoError(t, err)

	assert.Equal(t, testDevworkspace.Name, policy.OwnerReferences[0].Name)
	assert.Equal(t, testDevworkspace.UID, policy.OwnerReferences[0].UID)
	assert.Equal(t, "DevWorkspace", policy.OwnerReferences[0].Kind)
	assert.True(t, ptr.Deref(policy.OwnerReferences[0].Controller, false))
}

func TestSyncDeletesPolicyWhenDisabled(t *testing.T) {
	testDevworkspace := getTestDevWorkspaceWithConfig(testDevworkspaceName, getEnabledNetworkPolicyConfig())
	api := getTestClusterAPI(t, testDevworkspace.DevWorkspace)

	err := SyncNetworkPolicy(testDevworkspace, api)
	assert.Error(t, err)

	err = SyncNetworkPolicy(testDevworkspace, api)
	assert.NoError(t, err)

	_, err = getNetworkPolicyFromCluster(testDevworkspace, api)
	assert.NoError(t, err)

	testDevworkspace.Config.Workspace.NetworkPolicy.Enabled = new(false)
	err = SyncNetworkPolicy(testDevworkspace, api)
	assert.NoError(t, err)

	_, err = getNetworkPolicyFromCluster(testDevworkspace, api)
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestSyncIsTolerantOfMissingPolicyWhenDisabled(t *testing.T) {
	npConfig := getEnabledNetworkPolicyConfig()
	npConfig.Enabled = ptr.To(false)
	testdw := getTestDevWorkspaceWithConfig("test-devworkspace", npConfig)
	api := getTestClusterAPI(t, testdw.DevWorkspace)
	err := SyncNetworkPolicy(testdw, api)
	assert.NoError(t, err)
}
