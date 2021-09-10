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
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/utils/overriding"
	"github.com/devfile/api/v2/pkg/validation/variables"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten/network"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten/web_terminal"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DWTSupportedNamespacesAnnotation defines additional namespaces from which a DevWorkspace can import a DevWorkspaceTemplate.
	// By default, importing templates from the same namespace as the DevWorkspace is allowed.
	// Options are:
	// - '*': allow importing by all DevWorkspaces on the cluster
	// - 'namespaceA,namespaceB,namespaceC': Allow importing by DevWorkspaces in list of specific namespaces
	// If the annotation does not exist or is empty, only DevWorkspaces in the same namespace as the template can reference it.
	DWTSupportedNamespacesAnnotation = "controller.devfile.io/allow-import-from"
)

type ResolverTools struct {
	WorkspaceNamespace string
	Context            context.Context
	K8sClient          client.Client
	InternalRegistry   registry.InternalRegistry
	HttpClient         network.HTTPGetter
}

// ResolveDevWorkspace takes a devworkspace and returns a "resolved" version of it -- i.e. one where all plugins and parents
// are inlined as components.
func ResolveDevWorkspace(workspace *dw.DevWorkspaceTemplateSpec, tooling ResolverTools) (*dw.DevWorkspaceTemplateSpec, *variables.VariableWarning, error) {
	// Web terminals get default container components if they do not specify one
	if err := web_terminal.AddDefaultContainerIfNeeded(workspace); err != nil {
		return nil, nil, err
	}

	resolutionCtx := &resolutionContextTree{}
	resolvedDW, err := recursiveResolve(workspace, tooling, resolutionCtx)
	if err != nil {
		return nil, nil, err
	}

	warnings := variables.ValidateAndReplaceGlobalVariable(resolvedDW)
	if len(warnings.Commands) > 0 || len(warnings.Components) > 0 || len(warnings.Projects) > 0 || len(warnings.StarterProjects) > 0 {
		return resolvedDW, &warnings, nil
	}

	err = resolveWorkspaceEnvVar(resolvedDW)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process workspaceEnv: %w", err)
	}

	return resolvedDW, nil, nil
}

func recursiveResolve(workspace *dw.DevWorkspaceTemplateSpec, tooling ResolverTools, resolveCtx *resolutionContextTree) (*dw.DevWorkspaceTemplateSpec, error) {
	if DevWorkspaceIsFlattened(workspace) {
		return workspace.DeepCopy(), nil
	}

	resolvedParent := &dw.DevWorkspaceTemplateSpecContent{}
	if workspace.Parent != nil {
		resolvedParentSpec, err := resolveParentComponent(workspace.Parent, tooling)
		if err != nil {
			return nil, err
		}
		if !DevWorkspaceIsFlattened(resolvedParentSpec) {
			// TODO: implemenent this
			return nil, fmt.Errorf("parents containing plugins or parents are not supported")
		}
		annotate.AddSourceAttributesForTemplate("parent", resolvedParentSpec)
		resolvedParent = &resolvedParentSpec.DevWorkspaceTemplateSpecContent
	}
	resolvedContent := workspace.DevWorkspaceTemplateSpecContent.DeepCopy()
	resolvedContent.Components = nil

	var pluginSpecContents []*dw.DevWorkspaceTemplateSpecContent
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

			annotate.AddSourceAttributesForTemplate(component.Name, resolvedPlugin)
			pluginSpecContents = append(pluginSpecContents, &resolvedPlugin.DevWorkspaceTemplateSpecContent)
		}
	}

	if err := mergeVolumeComponents(resolvedContent, resolvedParent, pluginSpecContents...); err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace volumes: %w", err)
	}
	resolvedContent, err := overriding.MergeDevWorkspaceTemplateSpec(resolvedContent, resolvedParent, pluginSpecContents...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge DevWorkspace parents/plugins: %w", err)
	}

	return &dw.DevWorkspaceTemplateSpec{
		DevWorkspaceTemplateSpecContent: *resolvedContent,
	}, nil
}

// resolveParentComponent resolves the parent DevWorkspaceTemplateSpec that a parent reference refers to.
func resolveParentComponent(parent *dw.Parent, tools ResolverTools) (resolvedParent *dw.DevWorkspaceTemplateSpec, err error) {
	switch {
	case parent.Kubernetes != nil:
		// Search in default namespace if namespace ref is unset
		if parent.Kubernetes.Namespace == "" {
			parent.Kubernetes.Namespace = tools.WorkspaceNamespace
		}
		resolvedParent, err = resolveElementByKubernetesImport("parent", parent.Kubernetes, tools)
	case parent.Uri != "":
		resolvedParent, err = resolveElementByURI("parent", parent.Uri, tools)
	case parent.Id != "":
		resolvedParent, err = resolveElementById("parent", parent.Id, parent.RegistryUrl, tools)
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
	plugin *dw.PluginComponent,
	tools ResolverTools) (resolvedPlugin *dw.DevWorkspaceTemplateSpec, err error) {
	switch {
	case plugin.Kubernetes != nil:
		resolvedPlugin, err = resolveElementByKubernetesImport(name, plugin.Kubernetes, tools)
	case plugin.Uri != "":
		resolvedPlugin, err = resolveElementByURI(name, plugin.Uri, tools)
	case plugin.Id != "":
		resolvedPlugin, err = resolveElementById(name, plugin.Id, plugin.RegistryUrl, tools)
	default:
		err = fmt.Errorf("plugin %s does not define any resources", name)
	}
	if err != nil {
		return nil, err
	}

	if plugin.Components != nil || plugin.Commands != nil {
		overrideSpec, err := overriding.OverrideDevWorkspaceTemplateSpec(&resolvedPlugin.DevWorkspaceTemplateSpecContent, dw.PluginOverrides{
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
	kubeReference *dw.KubernetesCustomResourceImportReference,
	tools ResolverTools) (resolvedPlugin *dw.DevWorkspaceTemplateSpec, err error) {

	if tools.K8sClient == nil {
		return nil, fmt.Errorf("cannot resolve resources by kubernetes reference: no kubernetes client provided")
	}

	// Search in default namespace if namespace ref is unset
	namespace := kubeReference.Namespace
	if namespace == "" {
		if tools.WorkspaceNamespace == "" {
			return nil, fmt.Errorf("'%s' specifies a kubernetes reference without namespace and a default is not provided", name)
		}
		namespace = tools.WorkspaceNamespace
	}

	var dwTemplate dw.DevWorkspaceTemplate
	namespacedName := types.NamespacedName{
		Name:      kubeReference.Name,
		Namespace: namespace,
	}
	err = tools.K8sClient.Get(tools.Context, namespacedName, &dwTemplate)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("plugin for component %s not found", name)
		}
		return nil, fmt.Errorf("failed to retrieve plugin referenced by kubernetes name and namespace '%s': %w", name, err)
	}

	if !canImportDWT(tools.WorkspaceNamespace, &dwTemplate) {
		return nil, fmt.Errorf("could not find DevWorkspaceTemplate")
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
	tools ResolverTools) (resolvedPlugin *dw.DevWorkspaceTemplateSpec, err error) {

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

	if tools.HttpClient == nil {
		return nil, fmt.Errorf("cannot resolve resources by id: no HTTP client provided")
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
	tools ResolverTools) (resolvedPlugin *dw.DevWorkspaceTemplateSpec, err error) {

	if tools.HttpClient == nil {
		return nil, fmt.Errorf("cannot resolve resources by id: no HTTP client provided")
	}

	dwt, err := network.FetchDevWorkspaceTemplate(uri, tools.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve component %s by URI: %w", name, err)
	}
	return dwt, nil
}

// mergeVolumeComponents merges volume components sharing the same name according to the following rules
// * If a volume is defined in main and duplicated in parent/plugins, the copy in parent/plugins is removed
// * If a volume is defined in parent and duplicated in plugins, the copy in plugins is removed
// * If a volume is defined in multiple plugins, all but the first definition is removed
// * If a volume is defined as persistent, all duplicates will be persistent
// * If duplicate volumes set a size, the larger size will be used.
// Following the invocation of this function, there are no duplicate volumes defined across the main devworkspace, its
// parent, and its plugins.
func mergeVolumeComponents(main, parent *dw.DevWorkspaceTemplateSpecContent, plugins ...*dw.DevWorkspaceTemplateSpecContent) error {
	volumeComponents := map[string]dw.Component{}
	for _, component := range main.Components {
		if component.Volume == nil {
			continue
		}
		if _, exists := volumeComponents[component.Name]; exists {
			return fmt.Errorf("duplicate volume found in devfile: %s", component.Name)
		}
		volumeComponents[component.Name] = component
	}

	mergeVolumeComponents := func(spec *dw.DevWorkspaceTemplateSpecContent) error {
		var newComponents []dw.Component
		for _, component := range spec.Components {
			if component.Volume == nil {
				newComponents = append(newComponents, component)
				continue
			}
			if existingVol, exists := volumeComponents[component.Name]; exists {
				if err := mergeVolume(existingVol.Volume, component.Volume); err != nil {
					return err
				}
			} else {
				newComponents = append(newComponents, component)
				volumeComponents[component.Name] = component
			}
		}
		spec.Components = newComponents
		return nil
	}
	if err := mergeVolumeComponents(parent); err != nil {
		return err
	}

	for _, plugin := range plugins {
		if err := mergeVolumeComponents(plugin); err != nil {
			return err
		}
	}

	return nil
}

func mergeVolume(into, from *dw.VolumeComponent) error {
	// If the new volume is persistent, make the original persistent
	if !from.Ephemeral {
		into.Ephemeral = false
	}
	intoSize := into.Size
	if intoSize == "" {
		intoSize = "0"
	}
	intoSizeQty, err := resource.ParseQuantity(intoSize)
	if err != nil {
		return err
	}
	fromSize := from.Size
	if fromSize == "" {
		fromSize = "0"
	}
	fromSizeQty, err := resource.ParseQuantity(fromSize)
	if err != nil {
		return err
	}
	if fromSizeQty.Cmp(intoSizeQty) > 0 {
		into.Size = from.Size
	}
	return nil
}

// canImportDW returns true if a DevWorkspace in dwNamespace is allowed to reference the provided DevWorkspaceTemplate
// DevWorkspaces can by default only read DevWorkspaceTemplates in their own namespace, unless the DevWorkspaceTemplate
// has the controller.devfile.io/allow-import-from annotation.
func canImportDWT(dwNamespace string, dwt *dw.DevWorkspaceTemplate) bool {
	if dwNamespace == dwt.Namespace {
		return true
	}
	if dwt.Annotations == nil {
		return false
	}
	namespacesAnnotation := dwt.Annotations[DWTSupportedNamespacesAnnotation]
	switch namespacesAnnotation {
	case "":
		return false
	case "*":
		return true
	default:
		namespaces := strings.Split(namespacesAnnotation, ",")
		for _, ns := range namespaces {
			if ns == dwNamespace {
				return true
			}
		}
	}
	return false
}

func FormatVariablesWarning(warn *variables.VariableWarning) string {
	var msg []string
	for componentName, warnings := range warn.Components {
		msg = append(msg, fmt.Sprintf("invalid variables in component %s: '%s'", componentName, strings.Join(warnings, "', '")))
	}
	for commandName, warnings := range warn.Commands {
		msg = append(msg, fmt.Sprintf("invalid variables in component %s: '%s'", commandName, strings.Join(warnings, "', '")))
	}
	for projectName, warnings := range warn.Projects {
		msg = append(msg, fmt.Sprintf("invalid variables in project %s: '%s'", projectName, strings.Join(warnings, "', '")))
	}
	for starterProjectName, warnings := range warn.StarterProjects {
		msg = append(msg, fmt.Sprintf("invalid variables in starter project %s: '%s'", starterProjectName, strings.Join(warnings, "', '")))
	}
	return fmt.Sprintf("Error processing variable replacements: %s", strings.Join(msg, "; "))
}
