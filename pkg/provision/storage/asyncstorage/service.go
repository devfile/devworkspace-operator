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

package asyncstorage

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

func SyncWorkspaceSyncServiceToCluster(asyncDeploy *appsv1.Deployment, api wsprovision.ClusterAPI) (*corev1.Service, error) {
	specService := getWorkspaceSyncServiceSpec(asyncDeploy)
	err := controllerutil.SetOwnerReference(asyncDeploy, specService, api.Scheme)
	if err != nil {
		return nil, err
	}
	clusterService, err := getWorkspaceSyncServiceCluster(asyncDeploy.Namespace, api)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, err
		}
		// Service does not exist; create it.
		err := api.Client.Create(api.Ctx, specService)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return nil, err
		}
		return nil, NotReadyError
	}
	if !equality.Semantic.DeepDerivative(specService.Spec, clusterService.Spec) {
		// Delete service so that it can be recreated.
		err := api.Client.Delete(api.Ctx, clusterService)
		if err != nil && !k8sErrors.IsGone(err) {
			return nil, err
		}
		return nil, NotReadyError
	}
	return clusterService, nil
}

func getWorkspaceSyncServiceSpec(asyncDeploy *appsv1.Deployment) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      asyncServerServiceName,
			Namespace: asyncDeploy.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "async-storage", // TODO
				"app.kubernetes.io/part-of": "devworkspace-operator",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "rsync-port",
					Port:       rsyncPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(rsyncPort),
				},
			},
			Selector: asyncDeploy.Spec.Selector.MatchLabels,
		},
	}
}

func getWorkspaceSyncServiceCluster(namespace string, api wsprovision.ClusterAPI) (*corev1.Service, error) {
	service := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Name:      asyncServerServiceName,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, service)
	return service, err
}
