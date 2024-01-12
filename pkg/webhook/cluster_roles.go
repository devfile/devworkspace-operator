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

package webhook

import (
	"context"

	"github.com/devfile/devworkspace-operator/webhook/server"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateWebhookClusterRole(client crclient.Client,
	ctx context.Context) error {

	clusterRole, err := getSpecClusterRole()
	if err != nil {
		return err
	}

	if err := client.Create(ctx, clusterRole); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getExistingClusterRole(ctx, client)
		if err != nil {
			return err
		}
		clusterRole.ResourceVersion = existingCfg.ResourceVersion
		err = client.Update(ctx, clusterRole)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server cluster role")
	} else {
		log.Info("Created webhook server cluster role")
	}

	return nil
}

func getSpecClusterRole() (*v1.ClusterRole, error) {
	clusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   server.WebhookServerSAName,
			Labels: server.WebhookServerAppLabels(),
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{
					"admissionregistration.k8s.io",
				},
				Resources: []string{
					"mutatingwebhookconfigurations",
					"validatingwebhookconfigurations",
				},
				Verbs: []string{
					"create",
					"list",
					"watch",
					"get",
					"patch",
					"update",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"serviceaccounts",
				},
				Verbs: []string{
					"get",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"authentication.k8s.io",
				},
				Resources: []string{
					"tokenreviews",
				},
				Verbs: []string{
					"create",
				},
			},
			{
				APIGroups: []string{
					"authorization.k8s.io",
				},
				Resources: []string{
					"subjectaccessreviews",
					"localsubjectaccessreviews",
				},
				Verbs: []string{
					"create",
				},
			},
		},
	}

	return clusterRole, nil
}

func getExistingClusterRole(ctx context.Context, client crclient.Client) (*v1.ClusterRole, error) {
	clusterRole := &v1.ClusterRole{}
	namespacedName := types.NamespacedName{
		Name: server.WebhookServerSAName,
	}
	err := client.Get(ctx, namespacedName, clusterRole)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterRole, nil
}
