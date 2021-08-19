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
package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) ValidateDevfile(ctx context.Context, req admission.Request) admission.Response {

	wksp := &dwv2.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	workspace := &wksp.Spec.Template

	// flatten the devworkspace if its not already flattened
	if !flatten.DevWorkspaceIsFlattened(&wksp.Spec.Template) {
		flattenHelpers := flatten.ResolverTools{
			WorkspaceNamespace: wksp.Namespace,
			Context:            ctx,
			K8sClient:          h.Client,
			InternalRegistry:   &registry.InternalRegistryImpl{},
			HttpClient:         http.DefaultClient,
		}
		workspace, _, err = flatten.ResolveDevWorkspace(&wksp.Spec.Template, flattenHelpers)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	commands := workspace.Commands
	components := workspace.Components
	events := workspace.Events
	projects := workspace.Projects
	starterProjects := workspace.StarterProjects

	var devfileErrors []string

	// validate commands
	if commands != nil && components != nil {
		cmdErrors := devfilevalidation.ValidateCommands(commands, components)
		if cmdErrors != nil {
			devfileErrors = append(devfileErrors, cmdErrors.Error())
		}
	}

	// validate components
	if components != nil {
		componentErrors := devfilevalidation.ValidateComponents(components)
		if componentErrors != nil {
			devfileErrors = append(devfileErrors, componentErrors.Error())
		}
	}

	// validate events
	if events != nil {
		eventErrors := devfilevalidation.ValidateEvents(*events, commands)
		if eventErrors != nil {
			devfileErrors = append(devfileErrors, eventErrors.Error())
		}
	}

	// validate projects
	if projects != nil {
		projectsErrors := devfilevalidation.ValidateProjects(projects)
		if projectsErrors != nil {
			devfileErrors = append(devfileErrors, projectsErrors.Error())
		}
	}

	// validate starter projects
	if starterProjects != nil {
		starterProjectErrors := devfilevalidation.ValidateStarterProjects(starterProjects)
		if starterProjectErrors != nil {
			devfileErrors = append(devfileErrors, starterProjectErrors.Error())
		}
	}

	if len(devfileErrors) > 0 {
		return admission.Denied(fmt.Sprintf("\n%s\n", strings.Join(devfileErrors, "\n")))
	}

	return admission.Allowed("No Devfile errors were found")
}
