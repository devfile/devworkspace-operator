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

package provision

import (
	"context"
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	cmpopts.IgnoreFields(v1alpha1.WorkspaceRouting{}, "TypeMeta", "ObjectMeta", "Status"),
}

func SyncRoutingToCluster(
	workspace *devworkspace.DevWorkspace,
	components []v1alpha1.ComponentDescription,
	clusterAPI ClusterAPI) RoutingProvisioningStatus {

	specRouting, err := getSpecRouting(workspace, components, clusterAPI.Scheme)
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
		clusterRouting.Spec = specRouting.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterRouting)
		if err != nil {
			if errors.IsConflict(err) {
				return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Requeue: true}}
			}
			return RoutingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Err: err}}
		}
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true},
		}
	}

	if clusterRouting.Status.Phase == v1alpha1.RoutingFailed {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{FailStartup: true},
		}
	}
	if clusterRouting.Status.Phase != v1alpha1.RoutingReady {
		return RoutingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{
				Continue: false,
				Requeue:  false,
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
	workspace *devworkspace.DevWorkspace,
	componentDescriptions []v1alpha1.ComponentDescription,
	scheme *runtime.Scheme) (*v1alpha1.WorkspaceRouting, error) {

	endpoints := map[string]v1alpha1.EndpointList{}
	for _, desc := range componentDescriptions {
		componentEndpoints := desc.ComponentMetadata.Endpoints
		if componentEndpoints != nil && len(componentEndpoints) > 0 {
			endpoints[desc.Name] = append(endpoints[desc.Name], componentEndpoints...)
		}
	}

	routing := &v1alpha1.WorkspaceRouting{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("routing-%s", workspace.Status.WorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
			},
		},
		Spec: v1alpha1.WorkspaceRoutingSpec{
			WorkspaceId:   workspace.Status.WorkspaceId,
			RoutingClass:  v1alpha1.WorkspaceRoutingClass(workspace.Spec.RoutingClass),
			RoutingSuffix: config.ControllerCfg.GetRoutingSuffix(),
			Endpoints:     endpoints,
			PodSelector: map[string]string{
				config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
			},
		},
	}
	err := controllerutil.SetControllerReference(workspace, routing, scheme)
	if err != nil {
		return nil, err
	}

	return routing, nil
}

func getClusterRouting(name string, namespace string, client runtimeClient.Client) (*v1alpha1.WorkspaceRouting, error) {
	routing := &v1alpha1.WorkspaceRouting{}
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
