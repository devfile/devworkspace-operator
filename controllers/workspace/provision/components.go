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
	"errors"
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/component/cmd_terminal"
	"github.com/devfile/devworkspace-operator/pkg/adaptor"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_workspace")

type ComponentProvisioningStatus struct {
	ProvisioningStatus
	ComponentDescriptions []v1alpha1.ComponentDescription
}

var componentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1alpha1.Component{}, "TypeMeta", "ObjectMeta", "Status"),
}

func SyncComponentsToCluster(
	workspace *devworkspace.DevWorkspace, clusterAPI ClusterAPI) ComponentProvisioningStatus {
	specComponents, err := getSpecComponents(workspace, clusterAPI.Scheme)
	if err != nil {
		return ComponentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err, FailStartup: true},
		}
	}

	clusterComponents, err := getClusterComponents(workspace, clusterAPI.Client)
	if err != nil {
		return ComponentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	toCreate, toUpdate, toDelete := sortComponents(specComponents, clusterComponents)
	if len(toCreate) == 0 && len(toUpdate) == 0 && len(toDelete) == 0 {
		return checkComponentsReadiness(clusterComponents)
	}

	for _, component := range toCreate {
		err := clusterAPI.Client.Create(context.TODO(), &component)
		log.Info("Creating component", "component", component.Name)
		if err != nil {
			return ComponentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Err: err},
			}
		}
	}

	for _, component := range toUpdate {
		log.Info("Updating component", "component", component.Name)
		err := clusterAPI.Client.Update(context.TODO(), &component)
		if err != nil {
			if k8sErrors.IsConflict(err) {
				return ComponentProvisioningStatus{
					ProvisioningStatus: ProvisioningStatus{Requeue: true},
				}
			}
			return ComponentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Err: err},
			}
		}
	}

	for _, component := range toDelete {
		log.Info("Deleting component", "component", component.Name)
		err := clusterAPI.Client.Delete(context.TODO(), &component)
		if err != nil {
			return ComponentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Err: err},
			}
		}
	}

	return ComponentProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: false,
			Requeue:  true,
		},
	}
}

func checkComponentsReadiness(components []v1alpha1.Component) ComponentProvisioningStatus {
	var componentDescriptions []v1alpha1.ComponentDescription
	for _, component := range components {
		if !component.Status.Ready {
			return ComponentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{},
			}
		}
		componentDescriptions = append(componentDescriptions, component.Status.ComponentDescriptions...)
	}
	return ComponentProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{
			Continue: true,
		},
		ComponentDescriptions: componentDescriptions,
	}
}

func getSpecComponents(workspace *devworkspace.DevWorkspace, scheme *runtime.Scheme) ([]v1alpha1.Component, error) {
	dockerComponents, pluginComponents, err := adaptor.SortComponentsByType(workspace.Spec.Template.Components)
	if err != nil {
		return nil, err
	}

	if len(dockerComponents) == 0 {
		if cmd_terminal.ContainsCmdTerminalComponent(pluginComponents) {
			defaultContainer, err := config.ControllerCfg.GetDefaultTerminalDockerimage()
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to provision default dockerimage component for '%s'", cmd_terminal.CommandLineTerminalPublisherName))
				return nil, errors.New("configure dockerimage component or ask administrator to fix default one for " + cmd_terminal.CommandLineTerminalPublisherName)
			}
			dockerComponents = []devworkspace.Component{
				{
					Container: defaultContainer,
				},
			}
		}
	}

	var components []v1alpha1.Component
	if len(dockerComponents) > 0 {
		dockerResolver := v1alpha1.Component{
			ObjectMeta: v1.ObjectMeta{
				Name:      fmt.Sprintf("components-%s-%s", workspace.Status.WorkspaceId, "docker"),
				Namespace: workspace.Namespace,
				Labels: map[string]string{
					config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
				},
			},
			Spec: v1alpha1.WorkspaceComponentSpec{
				WorkspaceId: workspace.Status.WorkspaceId,
				Components:  dockerComponents,
				Commands:    workspace.Spec.Template.Commands,
			},
		}
		err = controllerutil.SetControllerReference(workspace, &dockerResolver, scheme)
		if err != nil {
			return nil, err
		}
		components = append(components, dockerResolver)
	}
	if len(pluginComponents) > 0 {
		pluginResolver := v1alpha1.Component{
			ObjectMeta: v1.ObjectMeta{
				Name:      fmt.Sprintf("components-%s-%s", workspace.Status.WorkspaceId, "plugins"),
				Namespace: workspace.Namespace,
				Labels: map[string]string{
					config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
				},
			},
			Spec: v1alpha1.WorkspaceComponentSpec{
				WorkspaceId: workspace.Status.WorkspaceId,
				Components:  pluginComponents,
				Commands:    workspace.Spec.Template.Commands,
			},
		}
		err = controllerutil.SetControllerReference(workspace, &pluginResolver, scheme)
		if err != nil {
			return nil, err
		}
		components = append(components, pluginResolver)
	}
	return components, nil
}

func getClusterComponents(workspace *devworkspace.DevWorkspace, client runtimeClient.Client) ([]v1alpha1.Component, error) {
	found := &v1alpha1.ComponentList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, workspace.Status.WorkspaceId))
	if err != nil {
		return nil, err
	}
	listOptions := &runtimeClient.ListOptions{
		Namespace:     workspace.Namespace,
		LabelSelector: labelSelector,
	}
	err = client.List(context.TODO(), found, listOptions)
	if err != nil {
		return nil, err
	}
	return found.Items, nil
}

func sortComponents(spec, cluster []v1alpha1.Component) (create, update, delete []v1alpha1.Component) {
	for _, clusterComponent := range cluster {
		if contains, _ := listContainsByName(clusterComponent, spec); !contains {
			delete = append(delete, clusterComponent)
		}
	}

	for _, specComponent := range spec {
		if contains, idx := listContainsByName(specComponent, cluster); contains {
			clusterComponent := cluster[idx]
			if !cmp.Equal(specComponent, clusterComponent, componentDiffOpts) {
				clusterComponent.Spec = specComponent.Spec
				update = append(update, clusterComponent)
			}
		} else {
			create = append(create, specComponent)
		}
	}
	return
}

func listContainsByName(query v1alpha1.Component, list []v1alpha1.Component) (exists bool, idx int) {
	for idx, listItem := range list {
		if query.Name == listItem.Name {
			return true, idx
		}
	}
	return false, -1
}
