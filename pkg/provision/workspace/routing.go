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
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/conversion"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RoutingProvisioningStatus struct {
	ProvisioningStatus
	PodAdditions     *v1alpha1.PodAdditions
	ExposedEndpoints map[string]v1alpha1.ExposedEndpointList
}

func SyncRoutingToCluster(
	workspace *dw.DevWorkspace,
	clusterAPI sync.ClusterAPI) RoutingProvisioningStatus {

	specRouting, err := getSpecRouting(workspace, clusterAPI.Scheme)
	if err != nil {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	clusterObj, err := sync.SyncObjectWithCluster(specRouting, clusterAPI)
	switch t := err.(type) {
	case nil:
		break
	case *sync.NotInSyncError:
		return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Requeue: true}}
	case *sync.UnrecoverableSyncError:
		return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{FailStartup: true, Err: t.Cause}}
	default:
		return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Err: err}}
	}

	clusterRouting := clusterObj.(*v1alpha1.DevWorkspaceRouting)
	if clusterRouting.Status.Phase == v1alpha1.RoutingFailed {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{FailStartup: true, Message: clusterRouting.Status.Message},
		}
	}
	if clusterRouting.Status.Phase != v1alpha1.RoutingReady {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Continue: false,
				Requeue:  false,
				Message:  clusterRouting.Status.Message,
			},
		}
	}

	return RoutingProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		PodAdditions:     clusterRouting.Status.PodAdditions,
		ExposedEndpoints: clusterRouting.Status.ExposedEndpoints,
	}
}

func getSpecRouting(
	workspace *dw.DevWorkspace,
	scheme *runtime.Scheme) (*v1alpha1.DevWorkspaceRouting, error) {

	endpoints := map[string]v1alpha1.EndpointList{}
	for _, component := range workspace.Spec.Template.Components {
		if component.Container == nil {
			continue
		}
		componentEndpoints := component.Container.Endpoints
		if len(componentEndpoints) > 0 {
			endpoints[component.Name] = append(endpoints[component.Name], conversion.ConvertAllDevfileEndpoints(componentEndpoints)...)
		}
	}

	var annotations map[string]string
	if val, ok := workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]; ok {
		annotations = maputils.Append(annotations, constants.DevWorkspaceRestrictedAccessAnnotation, val)
	}
	annotations = maputils.Append(annotations, constants.DevWorkspaceStartedStatusAnnotation, "true")

	// copy the annotations for the specific routingClass from the workspace object to the routing
	expectedAnnotationPrefix := workspace.Spec.RoutingClass + constants.RoutingAnnotationInfix
	for k, v := range workspace.GetAnnotations() {
		if strings.HasPrefix(k, expectedAnnotationPrefix) {
			annotations = maputils.Append(annotations, k, v)
		}
	}

	routingClass := workspace.Spec.RoutingClass
	if routingClass == "" {
		routingClass = config.Routing.DefaultRoutingClass
	}

	routing := &v1alpha1.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.DevWorkspaceRoutingName(workspace.Status.DevWorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
			},
			Annotations: annotations,
		},
		Spec: v1alpha1.DevWorkspaceRoutingSpec{
			DevWorkspaceId: workspace.Status.DevWorkspaceId,
			RoutingClass:   v1alpha1.DevWorkspaceRoutingClass(routingClass),
			Endpoints:      endpoints,
			PodSelector: map[string]string{
				constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
			},
		},
	}
	err := controllerutil.SetControllerReference(workspace, routing, scheme)
	if err != nil {
		return nil, err
	}

	return routing, nil
}
