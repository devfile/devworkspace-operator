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
	DefaultNamespace string
	Context          context.Context
	K8sClient        client.Client
	InternalRegistry registry.InternalRegistry
	HttpClient       network.HTTPGetter
}

// ResolveDevWorkspace takes a devworkspace and returns a "resolved" version of it -- i.e. one where all plugins and parents
// are inlined as components.
func ResolveDevWorkspace(workspace *devworkspace.DevWorkspaceTemplateSpec, tooling ResolverTools) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	// Web terminals get default container components if they do not specify one
	if err := web_terminal.AddDefaultContainerIfNeeded(workspace); err != nil {
		return nil, err
	}

	resolutionCtx := &resolutionContextTree{}
	resolvedDW, err := recursiveResolve(workspace, tooling, resolutionCtx)
	if err != nil {
		return nil, err
	}
	return resolvedDW, nil
}

func recursiveResolve(workspace *devworkspace.DevWorkspaceTemplateSpec, tooling ResolverTools, resolveCtx *resolutionContextTree) (*devworkspace.DevWorkspaceTemplateSpec, error) {
	if DevWorkspaceIsFlattened(workspace) {
		return workspace.DeepCopy(), nil
	}

	resolvedParent := &devworkspace.DevWorkspaceTemplateSpecContent{}
	if workspace.Parent != nil {
		resolvedParentSpec, err := resolveParentComponent(workspace.Parent, tooling)
		if err != nil {
			return nil, err
		}
		if !DevWorkspaceIsFlattened(resolvedParentSpec) {
			// TODO: implemenent this
			return nil, fmt.Errorf("parents containing plugins or parents are not supported")
		}
		resolvedParent = &resolvedParentSpec.DevWorkspaceTemplateSpecContent
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

			resolvedPlugin, err := recursiveResolve(pluginComponent, tooling, newCtx)
			if err != nil {
				return nil, err
			}

			annotate.AddSourceAttributesForPlugin(component.Name, resolvedPlugin)
			pluginSpecContents = append(pluginSpecContents, &resolvedPlugin.DevWorkspaceTemplateSpecContent)
		}
	}

	resolvedContent, err := overriding.MergeDevWorkspaceTemplateSpec(resolvedContent, resolvedParent, pluginSpecContents...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace parents/plugins: %w", err)
	}

	return &devworkspace.DevWorkspaceTemplateSpec{
		DevWorkspaceTemplateSpecContent: *resolvedContent,
	}, nil
}

// resolveParentComponent resolves the parent DevWorkspaceTemplateSpec that a parent reference refers to.
func resolveParentComponent(parent *devworkspace.Parent, tooling ResolverTools) (resolvedParent *devworkspace.DevWorkspaceTemplateSpec, err error) {
	switch {
	case parent.Kubernetes != nil:
		// Search in default namespace if namespace ref is unset
		if parent.Kubernetes.Namespace == "" {
			parent.Kubernetes.Namespace = tooling.DefaultNamespace
		}
		resolvedParent, err = resolveElementByKubernetesImport("parent", parent.Kubernetes, tooling)
	case parent.Uri != "":
		resolvedParent, err = resolveElementByURI("parent", parent.Uri, tooling)
	case parent.Id != "":
		resolvedParent, err = resolveElementById("parent", parent.Id, parent.RegistryUrl, tooling)
	default:
		err = fmt.Errorf("devfile parent does not define any resources")
	}
	if err != nil {
		return nil, err
	}
	if parent.Components != nil || parent.Commands != nil || parent.Projects != nil || parent.StarterProjects != nil {
		overrideSpec, err := overriding.OverrideDevWorkspaceTemplateSpec(&resolvedParent.DevWorkspaceTemplateSpecContent, parent.ParentOverrides)

		if err != nil {
			return nil, err
		}
		resolvedParent.DevWorkspaceTemplateSpecContent = *overrideSpec
	}
	return resolvedParent, nil
}

// resolvePluginComponent resolves the DevWorkspaceTemplateSpec that a plugin component refers to. The name parameter is
// used to construct meaningful error messages (e.g. issue resolving plugin 'name')
func resolvePluginComponent(
	name string,
	plugin *devworkspace.PluginComponent,
	tooling ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {
	switch {
	case plugin.Kubernetes != nil:
		// Search in default namespace if namespace ref is unset
		if plugin.Kubernetes.Namespace == "" {
			plugin.Kubernetes.Namespace = tooling.DefaultNamespace
		}
		resolvedPlugin, err = resolveElementByKubernetesImport(name, plugin.Kubernetes, tooling)
	case plugin.Uri != "":
		resolvedPlugin, err = resolveElementByURI(name, plugin.Uri, tooling)
	case plugin.Id != "":
		resolvedPlugin, err = resolveElementById(name, plugin.Id, plugin.RegistryUrl, tooling)
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

// resolveElementByKubernetesImport resolves a plugin specified by a Kubernetes reference.
// The name parameter is used to construct meaningful error messages (e.g. issue resolving plugin 'name')
func resolveElementByKubernetesImport(
	name string,
	kubeReference *devworkspace.KubernetesCustomResourceImportReference,
	tooling ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	var dwTemplate devworkspace.DevWorkspaceTemplate
	namespacedName := types.NamespacedName{
		Name:      kubeReference.Name,
		Namespace: kubeReference.Namespace,
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

// resolveElementById resolves a component specified by ID and registry URL. The name parameter is used to
// construct meaningful error messages (e.g. issue resolving plugin 'name'). When registry URL is empty,
// the DefaultRegistryURL from tools is used.
func resolveElementById(
	name string,
	id string,
	registryUrl string,
	tools ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	// Check internal registry for plugins that do not specify a registry
	if registryUrl == "" {
		if tools.InternalRegistry == nil {
			return nil, fmt.Errorf("plugin %s does not specify a registryUrl and no internal registry is configured", name)
		}
		if !tools.InternalRegistry.IsInInternalRegistry(id) {
			return nil, fmt.Errorf("plugin for component %s does not specify a registry and is not present in the internal registry", name)
		}
		pluginDWT, err := tools.InternalRegistry.ReadPluginFromInternalRegistry(id)
		if err != nil {
			return nil, fmt.Errorf("failed to read plugin for component %s from internal registry: %w", name, err)
		}
		return &pluginDWT.Spec, nil

	}
	pluginURL, err := url.Parse(registryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry URL for component %s: %w", name, err)
	}
	pluginURL.Path = path.Join(pluginURL.Path, id)

	dwt, err := network.FetchDevWorkspaceTemplate(pluginURL.String(), tools.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve component %s from registry %s: %w", name, registryUrl, err)
	}
	return dwt, nil
}

// resolveElementByURI resolves a plugin defined by URI. The name parameter is used to construct meaningful
// error messages (e.g. issue resolving plugin 'name')
func resolveElementByURI(
	name string,
	uri string,
	tools ResolverTools) (resolvedPlugin *devworkspace.DevWorkspaceTemplateSpec, err error) {

	dwt, err := network.FetchDevWorkspaceTemplate(uri, tools.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve component %s by URI: %w", name, err)
	}
	return dwt, nil
}
