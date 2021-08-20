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

	"github.com/devfile/devworkspace-operator/webhook/server"
)

const (
	ValidateWebhookCfgName       = "controller.devfile.io"
	validateWebhookPath          = "/validate"
	validateWebhookFailurePolicy = admregv1.Fail
)

func buildValidatingWebhookCfg(namespace string) *admregv1.ValidatingWebhookConfiguration {
	validateWebhookFailurePolicy := validateWebhookFailurePolicy
	validateWebhookPath := validateWebhookPath
	sideEffectsNone := admregv1.SideEffectClassNone
	return &admregv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ValidateWebhookCfgName,
			Labels: server.WebhookServerAppLabels(),
		},
		Webhooks: []admregv1.ValidatingWebhook{
			{
				Name:          "validate-exec.devworkspace-controller.svc",
				FailurePolicy: &validateWebhookFailurePolicy,
				SideEffects:   &sideEffectsNone,
				ClientConfig: admregv1.WebhookClientConfig{
					Service: &admregv1.ServiceReference{
						Name:      server.WebhookServerServiceName,
						Namespace: namespace,
						Path:      &validateWebhookPath,
					},
					CABundle: server.CABundle,
				},
				Rules: []admregv1.RuleWithOperations{
					{
						Operations: []admregv1.OperationType{admregv1.Connect},
						Rule: admregv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods/exec"},
						},
					},
				},
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
			},
		},
	}
}
