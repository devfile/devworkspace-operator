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

package flatten

import (
	"context"
	"fmt"
	"net/url"
	"path"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/utils/overriding"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten/network"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten/web_terminal"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResolverTools struct {
	InstanceNamespace string
	Context           context.Context
	K8sClient         client.Client
	InternalRegistry  registry.InternalRegistry
	HttpClient        network.HTTPGetter
}

// ResolveDevWorkspace takes a devworkspace and returns a "resolved" version of it -- i.e. one where all plugins and parents
// are inlined as components.
// TODO:
// - Implement flattening for DevWorkspace parents
// - Implement plugin references by ID and URI
func ResolveDevWorkspace(workspace devworkspace.DevWorkspaceTemplateSpec, tooling ResolverTools) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	// Web terminals get default container components if they do not specify one
	if err := web_terminal.AddDefaultContainerIfNeeded(&workspace); err != nil {
		return nil, err
	}

	resolutionCtx := &resolutionContextTree{}
	resolvedDW, err := recursiveResolve(workspace, tooling, resolutionCtx)
	if err != nil {
		return nil, err
	}
	return resolvedDW, nil
}

func recursiveResolve(workspace devworkspace.DevWorkspaceTemplateSpec, tooling ResolverTools, resolveCtx *resolutionContextTree) (*devworkspace.DevWorkspaceTemplateSpec, error) {
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
			newCtx := resolveCtx.addPlugin(component.Name, component.Plugin)
			if err := newCtx.hasCycle(); err != nil {
				return nil, err
			}

			resolvedPlugin, err := recursiveResolve(*pluginComponent, tooling, newCtx)
			if err != nil {
				return nil, err
			}

			annotate.AddSourceAttributesForPlugin(component.Name, resolvedPlugin)
			pluginSpecContents = append(pluginSpecContents, &resolvedPlugin.DevWorkspaceTemplateSpecContent)
		}
	}

	resolvedContent, err := overriding.MergeDevWorkspaceTemplateSpec(resolvedContent, nil, pluginSpecContents...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace parents/plugins: %w", err)
	}

	return &devworkspace.DevWorkspaceTemplateSpec{
		DevWorkspaceTemplateSpecContent: *resolvedContent,
	}, nil
}

func resolvePluginComponent(
	name string,
	plugin *devworkspace.PluginComponent,
	tooling ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {
	switch {
	// TODO: Add support for plugin ID and URI
	case plugin.Kubernetes != nil:
		// Search in devworkspace's namespace if namespace ref is unset
		if plugin.Kubernetes.Namespace == "" {
			plugin.Kubernetes.Namespace = tooling.InstanceNamespace
		}
		resolvedPlugin, err = resolvePluginComponentByKubernetesReference(name, plugin, tooling)
	case plugin.Uri != "":
		resolvedPlugin, err = resolvePluginComponentByURI(name, plugin, tooling)
	case plugin.Id != "":
		resolvedPlugin, err = resolvePluginComponentById(name, plugin, tooling)
	default:
		err = fmt.Errorf("plugin %s does not define any resources", name)
	}
	if err != nil {
		return nil, err
	}

	if plugin.Components != nil || plugin.Commands != nil {
		overrideSpec, err := overriding.OverrideDevWorkspaceTemplateSpec(&resolvedPlugin.DevWorkspaceTemplateSpecContent, devworkspace.PluginOverrides{
			Components: plugin.Components,
			Commands:   plugin.Commands,
		})

		if err != nil {
			return nil, err
		}
		resolvedPlugin.DevWorkspaceTemplateSpecContent = *overrideSpec
	}
	return resolvedPlugin, nil
}

func resolvePluginComponentByKubernetesReference(
	name string,
	plugin *devworkspace.PluginComponent,
	tooling ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	var dwTemplate devworkspace.DevWorkspaceTemplate
	namespacedName := types.NamespacedName{
		Name:      plugin.Kubernetes.Name,
		Namespace: plugin.Kubernetes.Namespace,
	}
	err = tooling.K8sClient.Get(tooling.Context, namespacedName, &dwTemplate)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("plugin for component %s not found", name)
		}
		return nil, fmt.Errorf("failed to retrieve plugin referenced by kubernetes name and namespace '%s': %w", name, err)
	}
	return &dwTemplate.Spec, nil
}

func resolvePluginComponentById(
	name string,
	plugin *devworkspace.PluginComponent,
	tools ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	// Check internal registry for plugins that do not specify a registry
	if plugin.RegistryUrl == "" {
		if tools.InternalRegistry == nil {
			return nil, fmt.Errorf("plugin %s does not specify a registryUrl and no internal registry is configured", name)
		}
		if !tools.InternalRegistry.IsInInternalRegistry(plugin.Id) {
			return nil, fmt.Errorf("plugin for component %s does not specify a registry and is not present in the internal registry", name)
		}
		pluginDWT, err := tools.InternalRegistry.ReadPluginFromInternalRegistry(plugin.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to read plugin for component %s from internal registry: %w", name, err)
		}
		return &pluginDWT.Spec, nil
	}

	pluginURL, err := url.Parse(plugin.RegistryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry URL for plugin %s: %w", name, err)
	}
	pluginURL.Path = path.Join(pluginURL.Path, "plugins", plugin.Id)

	dwt, err := network.FetchDevWorkspaceTemplate(pluginURL.String(), tools.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin %s from registry %s: %w", name, plugin.RegistryUrl, err)
	}
	return dwt, nil
}

func resolvePluginComponentByURI(
	name string,
	plugin *devworkspace.PluginComponent,
	tools ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	dwt, err := network.FetchDevWorkspaceTemplate(plugin.Uri, tools.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin %s by URI: %w", name, err)
	}
	return dwt, nil
}
