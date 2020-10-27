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

package restapis

import (
	"context"
	"encoding/json"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var configmapDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
}

func SyncRestAPIsConfigMap(workspace *devworkspace.DevWorkspace, components []v1alpha1.ComponentDescription, endpoints map[string]v1alpha1.ExposedEndpointList, clusterAPI provision.ClusterAPI) provision.ProvisioningStatus {
	specCM, err := getSpecConfigMap(workspace, components, endpoints, clusterAPI.Scheme)
	if err != nil {
		return provision.ProvisioningStatus{Err: err}
	}

	clusterCM, err := getClusterConfigMap(specCM.Name, workspace.Namespace, clusterAPI.Client)
	if err != nil {
		return provision.ProvisioningStatus{Err: err}
	}

	if clusterCM == nil {
		clusterAPI.Logger.Info("Creating che-rest-apis configmap")
		err := clusterAPI.Client.Create(context.TODO(), specCM)
		return provision.ProvisioningStatus{Requeue: true, Err: err}
	}

	if !cmp.Equal(specCM, clusterCM, configmapDiffOpts) {
		clusterAPI.Logger.Info("Updating che-rest-apis configmap")
		clusterCM.Data = specCM.Data
		err := clusterAPI.Client.Update(context.TODO(), clusterCM)
		if err != nil && !errors.IsConflict(err) {
			return provision.ProvisioningStatus{Err: err}
		}
		return provision.ProvisioningStatus{Requeue: true}
	}

	return provision.ProvisioningStatus{Continue: true}
}

func getSpecConfigMap(
	workspace *devworkspace.DevWorkspace,
	components []v1alpha1.ComponentDescription,
	endpoints map[string]v1alpha1.ExposedEndpointList,
	scheme *k8sRuntime.Scheme) (*corev1.ConfigMap, error) {
	runtimeJSON, err := constructRuntimeAnnotation(components, endpoints)
	if err != nil {
		return nil, err
	}
	devfileYAML, err := getDevfileV1Yaml(workspace.Spec.Template)
	if err != nil {
		return nil, err
	}

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.CheRestAPIsConfigmapName(workspace.Status.WorkspaceId),
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				config.WorkspaceIDLabel: workspace.Status.WorkspaceId,
			},
		},
		Data: map[string]string{
			config.RestAPIsDevfileYamlFilename: devfileYAML,
			config.RestAPIsRuntimeJSONFilename: runtimeJSON,
		},
	}
	err = controllerutil.SetControllerReference(workspace, configmap, scheme)
	return configmap, err
}

func getClusterConfigMap(name, namespace string, client runtimeClient.Client) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := client.Get(context.TODO(), namespacedName, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return cm, err
}

func getDevfileV1Yaml(template devworkspace.DevWorkspaceTemplateSpec) (string, error) {
	devfile := devworkspaceTemplateToDevfileV1(&template)
	devfileYaml, err := yaml.Marshal(devfile)
	if err != nil {
		return "", err
	}
	return string(devfileYaml), err
}

func constructRuntimeAnnotation(components []v1alpha1.ComponentDescription, endpoints map[string]v1alpha1.ExposedEndpointList) (string, error) {
	defaultEnv := "default"

	machines := getMachinesAnnotation(components, endpoints)
	commands := getWorkspaceCommands(components)

	runtime := v1alpha1.CheWorkspaceRuntime{
		ActiveEnv: defaultEnv,
		Commands:  commands,
		Machines:  machines,
	}

	runtimeJSON, err := json.Marshal(runtime)
	if err != nil {
		return "", err
	}
	return string(runtimeJSON), nil
}

func getMachinesAnnotation(components []v1alpha1.ComponentDescription, endpoints map[string]v1alpha1.ExposedEndpointList) map[string]v1alpha1.CheWorkspaceMachine {
	machines := map[string]v1alpha1.CheWorkspaceMachine{}
	machineRunningString := v1alpha1.RunningMachineEventType
	for _, component := range components {
		for containerName, container := range component.ComponentMetadata.Containers {
			servers := map[string]v1alpha1.CheWorkspaceServer{}
			// TODO: This is likely not a good choice for matching, since it'll fail if container name does not match an endpoint key
			for _, endpoint := range endpoints[containerName] {
				servers[endpoint.Name] = v1alpha1.CheWorkspaceServer{
					Attributes: endpoint.Attributes,
					Status:     v1alpha1.RunningServerStatus, // TODO: This is just set so the circles are green -- should check readiness
					URL:        endpoint.Url,
				}
			}
			machines[containerName] = v1alpha1.CheWorkspaceMachine{
				Attributes: container.Attributes,
				Servers:    servers,
				Status:     &machineRunningString,
			}
		}
	}

	return machines
}

func getWorkspaceCommands(components []v1alpha1.ComponentDescription) []v1alpha1.CheWorkspaceCommand {
	var commands []v1alpha1.CheWorkspaceCommand
	for _, component := range components {
		commands = append(commands, component.ComponentMetadata.ContributedRuntimeCommands...)
	}
	return commands
}
