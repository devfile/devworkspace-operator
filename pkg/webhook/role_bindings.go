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

package webhook

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/webhook/server"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookRoleBinding(client crclient.Client, ctx context.Context, namespace string) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	roleBinding := getSpecRoleBinding(namespace)
	if err := client.Create(ctx, roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingRoleBinding := &v1.RoleBinding{}
		if err := client.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: namespace}, existingRoleBinding); err != nil {
			return err
		}
		roleBinding.ResourceVersion = existingRoleBinding.ResourceVersion
		if err := client.Update(ctx, roleBinding); err != nil {
			return err
		}
		log.Info("Updated webhook server role binding")
	} else {
		log.Info("Created webhook server role binding")
	}

	return nil
}

func getSpecRoleBinding(namespace string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.WebhookServerSAName,
			Namespace: namespace,
			Labels:    server.WebhookServerAppLabels(),
		},
		Subjects: []v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      server.WebhookServerSAName,
				Namespace: namespace,
			},
		},
		RoleRef: v1.RoleRef{
			Kind:     "Role",
			Name:     server.WebhookServerSAName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}
