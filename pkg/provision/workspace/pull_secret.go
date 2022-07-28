//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"k8s.io/apimachinery/pkg/types"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	pullSecretCreationTimeout time.Duration = 5 * time.Second
)

type PullSecretsProvisioningStatus struct {
	ProvisioningStatus
	v1alpha1.PodAdditions
}

func PullSecrets(clusterAPI sync.ClusterAPI, serviceAccountName, namespace string) PullSecretsProvisioningStatus {
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.DevWorkspacePullSecretLabel, "true"))
	if err != nil {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err:         err,
				FailStartup: true,
			},
		}
	}

	secrets := corev1.SecretList{}
	err = clusterAPI.Client.List(context.TODO(), &secrets, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err: err,
			},
		}
	}

	serviceAccount := &corev1.ServiceAccount{}
	namespacedName := types.NamespacedName{
		Name:      serviceAccountName,
		Namespace: namespace,
	}
	err = clusterAPI.Client.Get(context.TODO(), namespacedName, serviceAccount)
	if err != nil {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err: err,
			},
		}
	}

	if len(serviceAccount.ImagePullSecrets) == 0 && serviceAccount.CreationTimestamp.Add(pullSecretCreationTimeout).After(time.Now()) {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Requeue: true,
				Message: "Waiting for image pull secrets",
			},
		}
	}

	dockerCfgs := serviceAccount.ImagePullSecrets
	for _, s := range secrets.Items {
		if s.Type == corev1.SecretTypeDockercfg || s.Type == corev1.SecretTypeDockerConfigJson {
			dockerCfgs = append(dockerCfgs, corev1.LocalObjectReference{Name: s.Name})
		}
	}

	sort.Slice(dockerCfgs, func(i, j int) bool {
		return strings.Compare(dockerCfgs[i].Name, dockerCfgs[j].Name) < 0
	})

	return PullSecretsProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		PodAdditions: v1alpha1.PodAdditions{
			PullSecrets: dockerCfgs,
		},
	}
}
