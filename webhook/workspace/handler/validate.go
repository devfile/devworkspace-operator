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

	commands := workspace.Commands
	events := workspace.Events
	projects := workspace.Projects
	starterProjects := workspace.StarterProjects

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
