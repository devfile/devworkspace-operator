//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	admregv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/webhook/server"
)

const (
	MutateWebhookCfgName       = "controller.devfile.io"
	mutateWebhookPath          = "/mutate"
	mutateWebhookFailurePolicy = admregv1.Fail
)

// BuildMutateWebhookCfg creates the mutating webhook configuration for the controller
func BuildMutateWebhookCfg(namespace string) *admregv1.MutatingWebhookConfiguration {
	mutateWebhookFailurePolicy := mutateWebhookFailurePolicy
	mutateWebhookPath := mutateWebhookPath
	labelExistsOp := metav1.LabelSelectorOpExists
	equivalentMatchPolicy := admregv1.Equivalent
	sideEffectsNone := admregv1.SideEffectClassNone
	webhookClientConfig := admregv1.WebhookClientConfig{
		Service: &admregv1.ServiceReference{
			Name:      server.WebhookServerServiceName,
			Namespace: namespace,
			Path:      &mutateWebhookPath,
		},
		CABundle: server.CABundle,
	}

	workspaceMutateWebhook := admregv1.MutatingWebhook{
		Name:          "mutate.devworkspace-controller.svc",
		FailurePolicy: &mutateWebhookFailurePolicy,
		ClientConfig:  webhookClientConfig,
		SideEffects:   &sideEffectsNone,
		Rules: []admregv1.RuleWithOperations{
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"workspace.devfile.io"},
					APIVersions: []string{"v1alpha1", "v1alpha2"},
					Resources:   []string{"devworkspaces"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"controller.devfile.io"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"devworkspaceroutings", "components"},
				},
			},
		},
		AdmissionReviewVersions: []string{"v1beta1", "v1"},
	}

	workspaceObjMutateWebhook := admregv1.MutatingWebhook{
		Name:          "mutate-ws-resources.devworkspace-controller.svc",
		FailurePolicy: &mutateWebhookFailurePolicy,
		ClientConfig:  webhookClientConfig,
		SideEffects:   &sideEffectsNone,
		ObjectSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      constants.DevWorkspaceIDLabel,
					Operator: labelExistsOp,
				},
			},
		},
		MatchPolicy: &equivalentMatchPolicy,
		Rules: []admregv1.RuleWithOperations{
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"services"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"networking"},
					APIVersions: []string{"v1"},
					Resources:   []string{"ingresses"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"networking.k8s.io"},
					APIVersions: []string{"v1"},
					Resources:   []string{"ingresses"},
				},
			},
			{
				Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
				Rule: admregv1.Rule{
					APIGroups:   []string{"batch"},
					APIVersions: []string{"v1"},
					Resources:   []string{"jobs"},
				},
			},
		},
		AdmissionReviewVersions: []string{"v1beta1", "v1"},
	}
	// n.b. Routes do not get UserInfo.UID filled in webhooks for some reason
	// ref: https://github.com/eclipse/che/issues/17114
	if infrastructure.IsOpenShift() {
		workspaceObjMutateWebhook.Rules = append(workspaceObjMutateWebhook.Rules, admregv1.RuleWithOperations{
			Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
			Rule: admregv1.Rule{
				APIGroups:   []string{"route.openshift.io"},
				APIVersions: []string{"v1"},
				Resources:   []string{"routes"},
			},
		})
	}

	return &admregv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   MutateWebhookCfgName,
			Labels: server.WebhookServerAppLabels(),
		},
		Webhooks: []admregv1.MutatingWebhook{
			workspaceMutateWebhook,
			workspaceObjMutateWebhook,
		},
	}
}
