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

	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/conversion"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
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
	workspaceWithConfig *common.DevWorkspaceWithConfig,
	clusterAPI sync.ClusterAPI) RoutingProvisioningStatus {

	specRouting, err := getSpecRouting(workspaceWithConfig, clusterAPI.Scheme)
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
	workspaceWithConfig *common.DevWorkspaceWithConfig,
	scheme *runtime.Scheme) (*v1alpha1.DevWorkspaceRouting, error) {

	endpoints := map[string]v1alpha1.EndpointList{}
	for _, component := range workspaceWithConfig.Spec.Template.Components {
		if component.Container == nil {
			continue
		}
		componentEndpoints := component.Container.Endpoints
		if len(componentEndpoints) > 0 {
			endpoints[component.Name] = append(endpoints[component.Name], conversion.ConvertAllDevfileEndpoints(componentEndpoints)...)
		}
	}

	var annotations map[string]string
	if val, ok := workspaceWithConfig.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]; ok {
		annotations = maputils.Append(annotations, constants.DevWorkspaceRestrictedAccessAnnotation, val)
	}
	annotations = maputils.Append(annotations, constants.DevWorkspaceStartedStatusAnnotation, "true")

	routingClass := workspaceWithConfig.Spec.RoutingClass
	if routingClass == "" {
		routingClass = workspaceWithConfig.Config.Routing.DefaultRoutingClass
	}

	// copy the annotations for the specific routingClass from the workspace object to the routing
	expectedAnnotationPrefix := routingClass + constants.RoutingAnnotationInfix
	for k, v := range workspaceWithConfig.GetAnnotations() {
		if strings.HasPrefix(k, expectedAnnotationPrefix) {
			annotations = maputils.Append(annotations, k, v)
		}
	}

	if v1alpha1.DevWorkspaceRoutingClass(routingClass) == v1alpha1.DevWorkspaceRoutingBasic {
		annotations = maputils.Append(annotations, constants.ClusterHostSuffixAnnotation, workspaceWithConfig.Config.Routing.ClusterHostSuffix)
	}

	routing := &v1alpha1.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.DevWorkspaceRoutingName(workspaceWithConfig.Status.DevWorkspaceId),
			Namespace: workspaceWithConfig.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: workspaceWithConfig.Status.DevWorkspaceId,
			},
			Annotations: annotations,
		},
		Spec: v1alpha1.DevWorkspaceRoutingSpec{
			DevWorkspaceId: workspaceWithConfig.Status.DevWorkspaceId,
			RoutingClass:   v1alpha1.DevWorkspaceRoutingClass(routingClass),
			Endpoints:      endpoints,
			PodSelector: map[string]string{
				constants.DevWorkspaceIDLabel: workspaceWithConfig.Status.DevWorkspaceId,
			},
		},
	}
	err := controllerutil.SetControllerReference(&workspaceWithConfig.DevWorkspace, routing, scheme)
	if err != nil {
		return nil, err
	}

	return routing, nil
}
