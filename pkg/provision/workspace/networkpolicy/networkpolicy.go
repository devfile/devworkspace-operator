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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

// generateNetworkPolicy builds the NetworkPolicy applied to a single DevWorkspace's pods.
// The name, labels, podSelector and policyTypes are owned by the operator; only the ingress
// and egress rules come from configuration. Both directions are always listed in policyTypes,
// so a direction whose configured rule list is empty denies all traffic in that direction.
func generateNetworkPolicy(workspace *common.DevWorkspaceWithConfig, npConfig *v1alpha1.NetworkPolicyConfig) *networkingv1.NetworkPolicy {
	workspaceId := workspace.Status.DevWorkspaceId
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NetworkPolicyName(workspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:   workspaceId,
				constants.DevWorkspaceNameLabel: workspace.Name,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.DevWorkspaceIDLabel: workspaceId,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}

	if len(npConfig.Ingress) > 0 {
		ingress := make([]networkingv1.NetworkPolicyIngressRule, len(npConfig.Ingress))
		for i, rule := range npConfig.Ingress {
			ruleCopy := rule.DeepCopy()
			normalizePorts(ruleCopy.Ports)
			ingress[i] = *ruleCopy
		}
		policy.Spec.Ingress = ingress
	}

	if len(npConfig.Egress) > 0 {
		egress := make([]networkingv1.NetworkPolicyEgressRule, len(npConfig.Egress))
		for i, rule := range npConfig.Egress {
			ruleCopy := rule.DeepCopy()
			normalizePorts(ruleCopy.Ports)
			egress[i] = *ruleCopy
		}
		policy.Spec.Egress = egress
	}

	return policy
}

// normalizePorts fills in the protocol the API server would default, so that the generated
// spec compares equal to the object stored on the cluster and does not trigger an endless
// update loop.
func normalizePorts(ports []networkingv1.NetworkPolicyPort) {
	for i := range ports {
		if ports[i].Protocol == nil {
			ports[i].Protocol = ptr.To(corev1.ProtocolTCP)
		}
	}
}

func ShouldProvision(workspace *common.DevWorkspaceWithConfig) bool {
	npConfig := workspace.Config.Workspace.NetworkPolicy
	if npConfig == nil {
		return false
	}
	return ptr.Deref(npConfig.Enabled, constants.DefaultNetworkPolicyEnabled)
}

func SyncNetworkPolicy(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	if !ShouldProvision(workspace) {
		return DeleteNetworkPolicy(workspace, api)
	}
	return CreateNetworkPolicy(workspace, api)
}

func CreateNetworkPolicy(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	specPolicy := generateNetworkPolicy(workspace, workspace.Config.Workspace.NetworkPolicy)
	if err := controllerutil.SetControllerReference(workspace.DevWorkspace, specPolicy, api.Scheme); err != nil {
		return &dwerrors.FailError{
			Message: "failed to set owner reference on workspace network policy",
			Err:     err,
		}
	}
	if _, err := sync.SyncObjectWithCluster(specPolicy, api); err != nil {
		return dwerrors.WrapSyncError(err)
	}
	return nil
}

// DeleteNetworkPolicy removes a DevWorkspace's NetworkPolicy if it exists. It is a no-op
// when no policy is present, so callers can invoke it unconditionally. Deleting the
// DevWorkspace itself does not require this call: the policy carries an ownerReference and
// is garbage collected with the workspace.
func DeleteNetworkPolicy(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	name := common.NetworkPolicyName(workspace.Status.DevWorkspaceId)
	policy := &networkingv1.NetworkPolicy{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: workspace.Namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, policy)
	switch {
	case err == nil:
		if err := api.Client.Delete(api.Ctx, policy); err != nil && !k8sErrors.IsNotFound(err) {
			return &dwerrors.RetryError{
				Message: fmt.Sprintf("failed to delete network policy %s in namespace %s", name, workspace.Namespace),
				Err:     err,
			}
		}
		api.Logger.Info("Deleted workspace network policy", "name", name, "namespace", workspace.Namespace)
		return nil
	case k8sErrors.IsNotFound(err):
		// Already deleted
		return nil
	default:
		return &dwerrors.RetryError{
			Message: fmt.Sprintf("failed to read network policy %s in namespace %s", name, workspace.Namespace),
			Err:     err,
		}
	}
}
