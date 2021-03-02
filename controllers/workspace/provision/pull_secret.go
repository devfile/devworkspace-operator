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

package provision

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PullSecretsProvisioningStatus struct {
	ProvisioningStatus
	v1alpha1.PodAdditions
}

func PullSecrets(clusterAPI ClusterAPI) PullSecretsProvisioningStatus {
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.DevWorkspacePullSecretLabel, "true"))
	if err != nil {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err:     err,
				Requeue: true,
			},
		}
	}

	secrets := corev1.SecretList{}
	err = clusterAPI.Client.List(context.TODO(), &secrets, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return PullSecretsProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Err:     err,
				Requeue: true,
			},
		}
	}

	var dockerCfgs []corev1.LocalObjectReference
	for _, s := range secrets.Items {
		if s.Type == corev1.SecretTypeDockercfg || s.Type == corev1.SecretTypeDockerConfigJson {
			dockerCfgs = append(dockerCfgs, corev1.LocalObjectReference{Name: s.Name})
		}
	}
	return PullSecretsProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		PodAdditions: v1alpha1.PodAdditions{
			PullSecrets: dockerCfgs,
		},
	}
}
