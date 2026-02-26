//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) ValidateDevfile(ctx context.Context, req admission.Request) admission.Response {

	wksp := &dwv2.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	workspace := &wksp.Spec.Template

	commands := workspace.Commands
	events := workspace.Events
	projects := workspace.Projects
	starterProjects := workspace.StarterProjects
	dependentProjects := workspace.DependentProjects

	var devfileErrors []string

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

	if dependentProjects != nil {
		dependentProjectsErrors := devfilevalidation.ValidateProjects(dependentProjects)
		if dependentProjectsErrors != nil {
			devfileErrors = append(devfileErrors, dependentProjectsErrors.Error())
		}
	}

	// validate starter projects
	if starterProjects != nil {
		starterProjectErrors := devfilevalidation.ValidateStarterProjects(starterProjects)
		if starterProjectErrors != nil {
			devfileErrors = append(devfileErrors, starterProjectErrors.Error())
		}
	}

	endpointErrors := h.validateEndpoints(ctx, wksp)
	if endpointErrors != nil {
		devfileErrors = append(devfileErrors, endpointErrors.Error())
	}

	if len(devfileErrors) > 0 {
		return admission.Denied(fmt.Sprintf("\n%s\n", strings.Join(devfileErrors, "\n")))
	}

	return admission.Allowed("No Devfile errors were found")
}

func (h *WebhookHandler) validateEndpoints(ctx context.Context, workspace *dwv2.DevWorkspace) error {
	discoverableEndpoints := map[string]bool{}
	for _, component := range workspace.Spec.Template.Components {
		if component.Container != nil {
			for _, endpoint := range component.Container.Endpoints {
				if endpoint.Attributes.GetBoolean(string(v1alpha1.DiscoverableAttribute), nil) {
					discoverableEndpoints[endpoint.Name] = true
				}
			}
		}
	}

	if len(discoverableEndpoints) == 0 {
		return nil
	}

	workspaceList := &dwv2.DevWorkspaceList{}
	if err := h.Client.List(ctx, workspaceList, client.InNamespace(workspace.Namespace)); err != nil {
		return err
	}

	for _, otherWorkspace := range workspaceList.Items {
		if otherWorkspace.UID == workspace.UID {
			continue
		}
		for _, component := range otherWorkspace.Spec.Template.Components {
			if component.Container != nil {
				for _, endpoint := range component.Container.Endpoints {
					if discoverableEndpoints[endpoint.Name] {
						return &solvers.ServiceConflictError{
							EndpointName:  endpoint.Name,
							WorkspaceName: otherWorkspace.Name,
						}
					}
				}
			}
		}
	}

	return nil
}
