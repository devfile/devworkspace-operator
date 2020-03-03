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
package creator

import (
	"github.com/che-incubator/che-workspace-operator/pkg/webhook/server"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mutateWebhookCfgName       = "mutate-workspace-admission-hooks"
	mutateWebhookPath          = "/mutate-workspaces"
	mutateWebhookFailurePolicy = v1beta1.Fail
)

func buildMutateWebhookCfg() *v1beta1.MutatingWebhookConfiguration {
	mutateWebhookFailurePolicy := mutateWebhookFailurePolicy
	mutateWebhookPath := mutateWebhookPath
	return &v1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutateWebhookCfgName,
		},
		Webhooks: []v1beta1.MutatingWebhook{
			{
				Name:          "mutate-workspaces.che-workspace-controller.svc",
				FailurePolicy: &mutateWebhookFailurePolicy,
				ClientConfig: v1beta1.WebhookClientConfig{
					Service: &v1beta1.ServiceReference{
						Name:      "workspace-controller",
						Namespace: "che-workspace-controller",
						Path:      &mutateWebhookPath,
					},
					CABundle: server.CABundle,
				},
				Rules: []v1beta1.RuleWithOperations{
					{
						Operations: []v1beta1.OperationType{v1beta1.Create, v1beta1.Update},
						Rule: v1beta1.Rule{
							APIGroups:   []string{"workspace.che.eclipse.org"},
							APIVersions: []string{"v1alpha1"},
							Resources:   []string{"workspaces"},
						},
					},
				},
			},
		},
	}
}
