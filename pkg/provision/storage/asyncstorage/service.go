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

package asyncstorage

import (
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncWorkspaceSyncServiceToCluster(asyncDeploy *appsv1.Deployment, api sync.ClusterAPI) (*corev1.Service, error) {
	specService := getWorkspaceSyncServiceSpec(asyncDeploy)
	err := controllerutil.SetOwnerReference(asyncDeploy, specService, api.Scheme)
	if err != nil {
		return nil, err
	}

	clusterObj, err := sync.SyncObjectWithCluster(specService, api)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return nil, NotReadyError
	case *sync.UnrecoverableSyncError:
		return nil, t
	default:
		return nil, err
	}
	clusterService := clusterObj.(*corev1.Service)
	return clusterService, nil
}

func getWorkspaceSyncServiceSpec(asyncDeploy *appsv1.Deployment) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      asyncServerServiceName,
			Namespace: asyncDeploy.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "async-storage", // TODO
				"app.kubernetes.io/part-of":   "devworkspace-operator",
				constants.DevWorkspaceIDLabel: "",
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
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func getWorkspaceSyncServiceCluster(namespace string, api sync.ClusterAPI) (*corev1.Service, error) {
	service := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Name:      asyncServerServiceName,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, service)
	return service, err
}
