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

package workspace

import (
	"context"
	"fmt"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RoutingProvisioningStatus struct {
	ProvisioningStatus
	PodAdditions     *v1alpha1.PodAdditions
	ExposedEndpoints map[string]v1alpha1.ExposedEndpointList
}

var routingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1alpha1.DevWorkspaceRouting{}, "TypeMeta", "Status"),
	// To ensure updates to annotations and labels are noticed, we need to ignore all fields in ObjectMeta
	// *except* labels and annotations.
	cmpopts.IgnoreFields(v1alpha1.DevWorkspaceRouting{},
		"ObjectMeta.Name",
		"ObjectMeta.GenerateName",
		"ObjectMeta.Namespace",
		"ObjectMeta.SelfLink",
		"ObjectMeta.UID",
		"ObjectMeta.ResourceVersion",
		"ObjectMeta.Generation",
		"ObjectMeta.CreationTimestamp",
		"ObjectMeta.DeletionTimestamp",
		"ObjectMeta.DeletionGracePeriodSeconds",
		"ObjectMeta.OwnerReferences",
		"ObjectMeta.Finalizers",
		"ObjectMeta.ClusterName",
		"ObjectMeta.ManagedFields"),
}

func SyncRoutingToCluster(
	workspace *dw.DevWorkspace,
	clusterAPI ClusterAPI) RoutingProvisioningStatus {

	specRouting, err := getSpecRouting(workspace, clusterAPI.Scheme)
	if err != nil {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	clusterRouting, err := getClusterRouting(specRouting.Name, specRouting.Namespace, clusterAPI.Client)
	if err != nil {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterRouting == nil {
		err := clusterAPI.Client.Create(context.TODO(), specRouting)
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	if specRouting.Spec.RoutingClass != clusterRouting.Spec.RoutingClass {
		err := clusterAPI.Client.Delete(context.TODO(), clusterRouting)
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	if !cmp.Equal(specRouting, clusterRouting, routingDiffOpts) {
		clusterRouting.Labels = specRouting.Labels
		clusterRouting.Annotations = specRouting.Annotations
		clusterRouting.Spec = specRouting.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterRouting)
		if err != nil && !errors.IsConflict(err) {
			return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Err: err}}
		}
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true},
		}
	}

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
			endpoints[component.Name] = append(endpoints[component.Name], componentEndpoints...)
		}
	}

	var annotations map[string]string
	if val, ok := workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]; ok {
		annotations = maputils.Append(annotations, constants.DevWorkspaceRestrictedAccessAnnotation, val)
	}

	// copy the annotations for the specific routingClass from the workspace object to the routing
	expectedAnnotationPrefix := workspace.Spec.RoutingClass + constants.RoutingAnnotationInfix
	for k, v := range workspace.GetAnnotations() {
		if strings.HasPrefix(k, expectedAnnotationPrefix) {
			annotations = maputils.Append(annotations, k, v)
		}
	}

	routingClass := workspace.Spec.RoutingClass
	if routingClass == "" {
		routingClass = config.ControllerCfg.GetDefaultRoutingClass()
	}

	routing := &v1alpha1.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("routing-%s", workspace.Status.DevWorkspaceId),
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

func getClusterRouting(name string, namespace string, client runtimeClient.Client) (*v1alpha1.DevWorkspaceRouting, error) {
	routing := &v1alpha1.DevWorkspaceRouting{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, routing)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return routing, nil
}
