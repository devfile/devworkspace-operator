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

package service

import (
	"context"

	"github.com/devfile/devworkspace-operator/webhook/server"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateSecureService(client crclient.Client, ctx context.Context, namespace string, annotations map[string]string) error {
	port := int32(443)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        server.WebhookServerServiceName,
			Namespace:   namespace,
			Labels:      server.WebhookServerAppLabels(),
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       server.WebhookServerPortName,
					Port:       port,
					Protocol:   "TCP",
					TargetPort: intstr.FromString(server.WebhookServerPortName),
				},
				{
					Name:       server.WebhookMetricsPortName,
					Port:       9443,
					Protocol:   "TCP",
					TargetPort: intstr.FromString(server.WebhookMetricsPortName),
				},
			},
			Selector: server.WebhookServerAppLabels(),
		},
	}

	if err := client.Create(ctx, service); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg, err := getClusterService(ctx, namespace, client)
		if err != nil {
			return err
		}

		// Cannot naively copy spec, as clusterIP is unmodifiable
		clusterIP := existingCfg.Spec.ClusterIP
		existingCfg.Spec = service.Spec
		existingCfg.Spec.ClusterIP = clusterIP

		err = client.Update(ctx, existingCfg)
		if err != nil {
			return err
		}
		log.Info("Updated webhook server service")
	} else {
		log.Info("Created webhook server service")
	}
	return nil
}

func getClusterService(ctx context.Context, namespace string, client crclient.Client) (*corev1.Service, error) {
	service := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      server.WebhookServerServiceName,
	}
	err := client.Get(ctx, namespacedName, service)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return service, nil
}
