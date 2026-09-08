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

func CreateWebhookRole(client crclient.Client, ctx context.Context, namespace string) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	role := getSpecRole(namespace)
	if err := client.Create(ctx, role); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingRole := &v1.Role{}
		if err := client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: namespace}, existingRole); err != nil {
			return err
		}
		role.ResourceVersion = existingRole.ResourceVersion
		if err := client.Update(ctx, role); err != nil {
			return err
		}
		log.Info("Updated webhook server role")
	} else {
		log.Info("Created webhook server role")
	}

	return nil
}

func getSpecRole(namespace string) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.WebhookServerSAName,
			Namespace: namespace,
			Labels:    server.WebhookServerAppLabels(),
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"route.openshift.io",
				},
				Resources: []string{
					"routes",
				},
				Verbs: []string{
					"create",
					"get",
					"delete",
				},
			},
		},
	}
}
