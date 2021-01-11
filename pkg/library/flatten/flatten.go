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

package flatten

import (
	"context"
	"fmt"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/pkg/utils/overriding"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResolverTools struct {
	Context   context.Context
	K8sClient client.Client
}

// TODO: temp workaround for panic in devfile/api when using plugin overrides. See: https://github.com/devfile/api/issues/296
type tempOverrides struct {
	devworkspace.PluginOverrides
}

func (t tempOverrides) GetToplevelLists() devworkspace.TopLevelLists {
	base := t.PluginOverrides.GetToplevelLists()
	base["Projects"] = []devworkspace.Keyed{}
	base["StarterProjects"] = []devworkspace.Keyed{}
	return base
}

// END WORKAROUND

// ResolveDevWorkspace takes a devworkspace and returns a "resolved" version of it -- i.e. one where all plugins and parents
// are inlined as components.
// TODO:
// - Implement flattening for DevWorkspace parents
// - Implement plugin references by ID and URI
// - Implement plugin + editor compatibility checking
// - Implement cycle checking for references
func ResolveDevWorkspace(workspace devworkspace.DevWorkspaceTemplateSpec, tooling ResolverTools) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	if DevWorkspaceIsFlattened(workspace) {
		return workspace.DeepCopy(), nil
	}
	if workspace.Parent != nil {
		// TODO: Add support for flattening DevWorkspace parents
		return nil, fmt.Errorf("DevWorkspace parent is unsupported")
	}
	resolvedContent := &devworkspace.DevWorkspaceTemplateSpecContent{}
	resolvedContent.Projects = workspace.Projects
	resolvedContent.StarterProjects = workspace.StarterProjects
	resolvedContent.Commands = workspace.Commands
	resolvedContent.Events = workspace.Events

	var pluginSpecContents []*devworkspace.DevWorkspaceTemplateSpecContent
	for _, component := range workspace.Components {
		if component.Plugin == nil {
			// No action necessary
			resolvedContent.Components = append(resolvedContent.Components, component)
		} else {
			pluginComponent, err := resolvePluginComponent(component.Name, component.Plugin, tooling)
			if err != nil {
				return nil, err
			}
			resolvedPlugin, err := ResolveDevWorkspace(*pluginComponent, tooling)
			if err != nil {
				return nil, err
			}
			pluginSpecContents = append(pluginSpecContents, &resolvedPlugin.DevWorkspaceTemplateSpecContent)
		}
	}
	// TODO: Temp workaround for issue in devfile API: can't pass in nil for parentFlattenedContent
	// see: https://github.com/devfile/api/issues/295
	resolvedContent, err := overriding.MergeDevWorkspaceTemplateSpec(resolvedContent, &devworkspace.DevWorkspaceTemplateSpecContent{}, pluginSpecContents...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace parents/plugins: %w", err)
	}
	return &devworkspace.DevWorkspaceTemplateSpec{
		DevWorkspaceTemplateSpecContent: *resolvedContent,
	}, nil
}

func resolvePluginComponent(name string, plugin *devworkspace.PluginComponent, tooling ResolverTools) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	var resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec
	var err error
	switch {
	// TODO: Add support for plugin ID and URI
	case plugin.Kubernetes != nil:
		resolvedPlugin, err = resolvePluginComponentByKubernetesReference(name, plugin, tooling)
	case plugin.Uri != "":
		return nil, fmt.Errorf("failed to resolve plugin %s: only plugins specified by kubernetes reference are supported", name)
	case plugin.Id != "":
		return nil, fmt.Errorf("failed to resolve plugin %s: only plugins specified by kubernetes reference are supported", name)
	default:
		return nil, fmt.Errorf("plugin %s does not define any resources", name)
	}
	if err != nil {
		return nil, err
	}

	if plugin.Components != nil || plugin.Commands != nil {
		// TODO: temp workaround for panic in devfile/api when using plugin overrides. See: https://github.com/devfile/api/issues/296
		//overrideSpec, err := overriding.OverrideDevWorkspaceTemplateSpec(&resolvedPlugin.DevWorkspaceTemplateSpecContent, devworkspace.PluginOverrides{
		//	Components: plugin.Components,
		//	Commands:   plugin.Commands,
		//})
		overrideSpec, err := overriding.OverrideDevWorkspaceTemplateSpec(&resolvedPlugin.DevWorkspaceTemplateSpecContent, tempOverrides{
			PluginOverrides: devworkspace.PluginOverrides{
				Components: plugin.Components,
				Commands:   plugin.Commands,
			},
		})

		if err != nil {
			return nil, err
		}
		resolvedPlugin.DevWorkspaceTemplateSpecContent = *overrideSpec
	}
	return resolvedPlugin, nil
}

func resolvePluginComponentByKubernetesReference(name string, plugin *devworkspace.PluginComponent, tooling ResolverTools) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	var dwTemplate devworkspace.DevWorkspaceTemplate
	namespacedName := types.NamespacedName{
		Name:      plugin.Kubernetes.Name,
		Namespace: plugin.Kubernetes.Namespace,
	}
	err := tooling.K8sClient.Get(tooling.Context, namespacedName, &dwTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve referenced kubernetes name and namespace for plugin %s: %w", name, err)
	}
	return &dwTemplate.Spec, nil
}
