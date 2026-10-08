//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/common"
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

	names := discoverableEndpointNames(wksp)
	if h.shouldCheckEndpointConflicts(req, names) {
		conflict, err := h.validateEndpoints(ctx, wksp, names)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if conflict != nil {
			devfileErrors = append(devfileErrors, conflict.Error())
		}
	}

	if len(devfileErrors) > 0 {
		return admission.Denied(fmt.Sprintf("\n%s\n", strings.Join(devfileErrors, "\n")))
	}

	return admission.Allowed("No Devfile errors were found")
}

// shouldCheckEndpointConflicts reports whether validateEndpoints needs to run for this request. Newly created
// workspaces with discoverable endpoints need the check, since every discoverable endpoint is new. An empty set
// never needs the check. For updates, a new conflict can only be introduced by a discoverable endpoint name that
// wasn't already present, so the check is skipped unless the new set contains a name that wasn't in the old set.
// This also skips pure removals, which can never introduce a conflict and would otherwise re-reject unrelated edits
// blocked only by a pre-existing conflict on an endpoint the update doesn't touch.
func (h *WebhookHandler) shouldCheckEndpointConflicts(req admission.Request, names map[string]bool) bool {
	if len(names) == 0 {
		return false
	}

	if req.Operation != admissionv1.Update {
		return true
	}

	oldWorkspace := &dwv2.DevWorkspace{}
	if err := h.Decoder.DecodeRaw(req.OldObject, oldWorkspace); err != nil {
		return true
	}

	oldNames := discoverableEndpointNames(oldWorkspace)
	for name := range names {
		if !oldNames[name] {
			return true
		}
	}
	return false
}

// discoverableEndpointNames returns the sanitized Service names (see common.EndpointName) of the workspace's
// discoverable endpoints whose exposure is not none. Omitted exposure defaults to public and is included.
// Comparing by the sanitized name, rather than the raw devfile endpoint name, matches the
// actual collision surface: two differently-named endpoints that sanitize to the same value produce the same
// Service name and conflict at reconcile time.
//
// Endpoints contributed via workspace.Spec.Contributions (plugins/parent templates) are invisible here,
// permanently: flatten.ResolveDevWorkspace inlines them only in-memory during reconcile, never back to
// .spec.template, so this field never reflects them, no matter how many times a workspace reconciles. Flattening
// here instead isn't safe either - it requires cluster/network calls unsuitable for a webhook's time budget.
// Such conflicts still surface at reconcile time via Service synchronization, which checks the
// actual Service objects rather than any spec - the same behavior that existed before this check was added.
func discoverableEndpointNames(workspace *dwv2.DevWorkspace) map[string]bool {
	discoverableEndpoints := map[string]bool{}
	for _, component := range workspace.Spec.Template.Components {
		if component.Container != nil {
			for _, endpoint := range component.Container.Endpoints {
				if endpoint.Exposure != dwv2.NoneEndpointExposure && endpoint.Attributes.GetBoolean(string(v1alpha1.DiscoverableAttribute), nil) {
					discoverableEndpoints[common.EndpointName(endpoint.Name)] = true
				}
			}
		}
	}
	return discoverableEndpoints
}

// validateEndpoints is best-effort: it reads the current cluster state at admission time, so two concurrent updates
// racing to claim the same endpoint name could both be admitted. Service synchronization remains the authoritative
// ownership guard against that race.
func (h *WebhookHandler) validateEndpoints(ctx context.Context, workspace *dwv2.DevWorkspace, names map[string]bool) (*solvers.ServiceConflictError, error) {
	if len(names) == 0 {
		return nil, nil
	}

	workspaceList := &dwv2.DevWorkspaceList{}
	if err := h.Client.List(ctx, workspaceList, client.InNamespace(workspace.Namespace)); err != nil {
		return nil, err
	}

	for _, otherWorkspace := range workspaceList.Items {
		if otherWorkspace.UID == workspace.UID {
			continue
		}
		for _, component := range otherWorkspace.Spec.Template.Components {
			if component.Container != nil {
				for _, endpoint := range component.Container.Endpoints {
					if endpoint.Exposure != dwv2.NoneEndpointExposure &&
						endpoint.Attributes.Exists(string(v1alpha1.DiscoverableAttribute)) &&
						names[common.EndpointName(endpoint.Name)] &&
						endpoint.Attributes.GetBoolean(string(v1alpha1.DiscoverableAttribute), nil) {
						return &solvers.ServiceConflictError{
							EndpointName:  endpoint.Name,
							WorkspaceName: otherWorkspace.Name,
						}, nil
					}
				}
			}
		}
	}

	return nil, nil
}
