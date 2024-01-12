//
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
			{
				Name:          "validate-devfile.devworkspace-controller.svc",
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
						Operations: []admregv1.OperationType{admregv1.Create, admregv1.Update},
						Rule: admregv1.Rule{
							APIGroups:   []string{"workspace.devfile.io"},
							APIVersions: []string{"v1alpha2"},
							Resources:   []string{"devworkspaces"},
						},
					},
				},
				AdmissionReviewVersions: []string{"v1beta1", "v1"},
			},
		},
	}
}
