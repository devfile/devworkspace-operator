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

package workspace

import (
	"strings"
	"time"

	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/conversion"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncRoutingToCluster(
	workspace *common.DevWorkspaceWithConfig,
	clusterAPI sync.ClusterAPI) (*v1alpha1.PodAdditions, map[string]v1alpha1.ExposedEndpointList, string, error) {

	specRouting, err := getSpecRouting(workspace, clusterAPI.Scheme)
	if err != nil {
		return nil, nil, "", err
	}

	clusterObj, err := sync.SyncObjectWithCluster(specRouting, clusterAPI)
	if err != nil {
		return nil, nil, "", dwerrors.WrapSyncError(err)
	}

	clusterRouting := clusterObj.(*v1alpha1.DevWorkspaceRouting)
	statusMsg := clusterRouting.Status.Message
	if clusterRouting.Status.Phase == v1alpha1.RoutingFailed {
		return nil, nil, statusMsg, &dwerrors.FailError{Message: statusMsg}
	}
	if clusterRouting.Status.Phase != v1alpha1.RoutingReady {
		return nil, nil, statusMsg, &dwerrors.RetryError{Message: statusMsg, RequeueAfter: 5 * time.Second}
	}

	return clusterRouting.Status.PodAdditions, clusterRouting.Status.ExposedEndpoints, statusMsg, nil
}

func getSpecRouting(
	workspace *common.DevWorkspaceWithConfig,
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
		routingClass = workspace.Config.Routing.DefaultRoutingClass
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
	err := controllerutil.SetControllerReference(workspace.DevWorkspace, routing, scheme)
	if err != nil {
		return nil, err
	}

	return routing, nil
}
