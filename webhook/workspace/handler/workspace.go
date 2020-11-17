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

	devworkspacev1alpha1 "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	devworkspacev1alpha2 "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &devworkspacev1alpha1.DevWorkspace{}
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

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &devworkspacev1alpha2.DevWorkspace{}
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

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &devworkspacev1alpha1.DevWorkspace{}
	oldWksp := &devworkspacev1alpha1.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	allowed, msg := h.handleImmutableWorkspaceV1alpha1(oldWksp, newWksp, req.UserInfo.UID)
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

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &devworkspacev1alpha2.DevWorkspace{}
	oldWksp := &devworkspacev1alpha2.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	allowed, msg := h.handleImmutableWorkspaceV1alpha2(oldWksp, newWksp, req.UserInfo.UID)
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
