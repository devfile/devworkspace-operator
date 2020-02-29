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

package registry

import (
	"context"
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/ownerref"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("che-plugin-registry")

var EmbeddedPluginRegistryUrl = ""

const (
	// PrometheusPortName defines the port name used in the metrics Service.
	PrometheusPortName = "metrics"
)

// ExposeRegistryPort creates a Kubernetes Service to expose the passed registry port.
func ExposeRegistryPort(ctx context.Context, port int32) (*v1.Service, error) {
	client, err := createClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create new client: %v", err)
	}
	// We do not need to check the validity of the port, as controller-runtime
	// would error out and we would never get to this stage.
	s, err := initOperatorService(ctx, client, port, "registry")
	if err != nil {
		if err == k8sutil.ErrNoNamespace {
			log.Info("Skipping plugin registry Service creation; not running in a cluster.")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to initialize service object for plugin registry: %v", err)
	}
	service, err := createOrUpdateService(ctx, client, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create or get service for plugin registry: %v", err)
	}

	EmbeddedPluginRegistryUrl = "http://" + s.Name + "." + s.Namespace + ".svc.cluster.local:8080/v3"
	return service, nil
}

func createOrUpdateService(ctx context.Context, client crclient.Client, s *v1.Service) (*v1.Service, error) {
	if err := client.Create(ctx, s); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		// Service already exists, we want to update it
		// as we do not know if any fields might have changed.
		existingService := &v1.Service{}
		err := client.Get(ctx, types.NamespacedName{
			Name:      s.Name,
			Namespace: s.Namespace,
		}, existingService)

		s.ResourceVersion = existingService.ResourceVersion
		if existingService.Spec.Type == v1.ServiceTypeClusterIP {
			s.Spec.ClusterIP = existingService.Spec.ClusterIP
		}
		err = client.Update(ctx, s)
		if err != nil {
			return nil, err
		}
		log.V(1).Info("Metrics Service object updated", "Service.Name", s.Name, "Service.Namespace", s.Namespace)
		return existingService, nil
	}

	log.Info("Metrics Service object created", "Service.Name", s.Name, "Service.Namespace", s.Namespace)
	return s, nil
}

// initOperatorService returns the static service which exposes specified port.
func initOperatorService(ctx context.Context, client crclient.Client, port int32, portName string) (*v1.Service, error) {
	operatorName, err := k8sutil.GetOperatorName()
	if err != nil {
		return nil, err
	}
	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	label := map[string]string{"name": operatorName}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName + "-plugin-registry",
			Namespace: namespace,
			Labels:    label,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port:     port,
					Protocol: v1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: port,
					},
					Name: portName,
				},
			},
			Selector: label,
		},
	}

	ownRef, err := ownerref.FindControllerOwner(ctx, client)
	if err != nil {
		return nil, err
	}
	service.SetOwnerReferences([]metav1.OwnerReference{*ownRef})

	return service, nil
}

func createClient() (crclient.Client, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	client, err := crclient.New(config, crclient.Options{})
	if err != nil {
		return nil, err
	}

	return client, nil
}
