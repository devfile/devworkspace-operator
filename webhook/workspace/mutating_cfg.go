//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package workspace

import (
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/webhook/server"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MutateWebhookCfgName       = "controller.devfile.io"
	mutateWebhookPath          = "/mutate"
	mutateWebhookFailurePolicy = v1beta1.Fail
)

// BuildMutateWebhookCfg creates the mutating webhook configuration for the controller
func BuildMutateWebhookCfg(namespace string) *v1beta1.MutatingWebhookConfiguration {
	mutateWebhookFailurePolicy := mutateWebhookFailurePolicy
	mutateWebhookPath := mutateWebhookPath
	labelExistsOp := metav1.LabelSelectorOpExists
	equivalentMatchPolicy := v1beta1.Equivalent
	webhookClientConfig := v1beta1.WebhookClientConfig{
		Service: &v1beta1.ServiceReference{
			Name:      server.WebhookServerServiceName,
			Namespace: namespace,
			Path:      &mutateWebhookPath,
		},
		CABundle: server.CABundle,
	}

	workspaceMutateWebhook := v1beta1.MutatingWebhook{
		Name:          "mutate.devworkspace-controller.svc",
		FailurePolicy: &mutateWebhookFailurePolicy,
		ClientConfig:  webhookClientConfig,
		Rules: []v1beta1.RuleWithOperations{
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{"workspace.devfile.io"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"devworkspaces"},
				},
			},
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{"controller.devfile.io"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"workspaceroutings", "components"},
				},
			},
		},
	}

	workspaceObjMutateWebhook := v1beta1.MutatingWebhook{
		Name:          "mutate-ws-resources.devworkspace-controller.svc",
		FailurePolicy: &mutateWebhookFailurePolicy,
		ClientConfig:  webhookClientConfig,
		ObjectSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      config.WorkspaceIDLabel,
					Operator: labelExistsOp,
				},
			},
		},
		MatchPolicy: &equivalentMatchPolicy,
		Rules: []v1beta1.RuleWithOperations{
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"services"},
				},
			},
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments"},
				},
			},
			{
				Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
				Rule: v1beta1.Rule{
					APIGroups:   []string{"extensions"},
					APIVersions: []string{"v1beta1"},
					Resources:   []string{"ingresses"},
				},
			},
		},
	}
	// n.b. Routes do not get UserInfo.UID filled in webhooks for some reason
	// ref: https://github.com/eclipse/che/issues/17114
	if config.ControllerCfg.IsOpenShift() {
		workspaceObjMutateWebhook.Rules = append(workspaceObjMutateWebhook.Rules, v1beta1.RuleWithOperations{
			Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
			Rule: v1beta1.Rule{
				APIGroups:   []string{"route.openshift.io"},
				APIVersions: []string{"v1"},
				Resources:   []string{"routes"},
			},
		})
	}

	return &v1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   MutateWebhookCfgName,
			Labels: server.WebhookServerAppLabels(),
		},
		Webhooks: []v1beta1.MutatingWebhook{
			workspaceMutateWebhook,
			workspaceObjMutateWebhook,
		},
	}
}
