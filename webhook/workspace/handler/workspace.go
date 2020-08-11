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
package handler

import (
	"context"
	"fmt"
	"net/http"

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceOnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &devworkspace.DevWorkspace{}

	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if wksp.Labels == nil {
		wksp.Labels = map[string]string{}
	}
	wksp.Labels[config.WorkspaceCreatorLabel] = req.UserInfo.UID
	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceOnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &devworkspace.DevWorkspace{}
	oldWksp := &devworkspace.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	allowed, msg := h.handleImmutableWorkspace(oldWksp, newWksp, req.UserInfo.UID)
	if !allowed {
		return admission.Denied(msg)
	}

	oldCreator, found := oldWksp.Labels[config.WorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate workspace to get it initialized", config.WorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[config.WorkspaceCreatorLabel]
	if !found {
		newWksp.Labels[config.WorkspaceCreatorLabel] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once workspace is created and is immutable", config.WorkspaceCreatorLabel))
	}

	return admission.Allowed("new workspace has the same workspace as old one")
}
