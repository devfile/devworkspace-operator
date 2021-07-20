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

	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &dwv1.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	wksp.Labels = maputils.Append(wksp.Labels, constants.DevWorkspaceCreatorLabel, req.UserInfo.UID)

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &dwv2.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	wksp.Labels = maputils.Append(wksp.Labels, constants.DevWorkspaceCreatorLabel, req.UserInfo.UID)

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &dwv1.DevWorkspace{}
	oldWksp := &dwv1.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	allowed, msg := h.checkRestrictedAccessWorkspaceV1alpha1(oldWksp, newWksp, req.UserInfo.UID)
	if !allowed {
		return admission.Denied(msg)
	}

	oldCreator, found := oldWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate devworkspace to get it initialized", constants.DevWorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		if newWksp.Labels == nil {
			newWksp.Labels = map[string]string{}
		}
		newWksp.Labels[constants.DevWorkspaceCreatorLabel] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel))
	}

	return admission.Allowed("new devworkspace has the same devworkspace creator as old one")
}

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &dwv2.DevWorkspace{}
	oldWksp := &dwv2.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	allowed, msg := h.checkRestrictedAccessWorkspaceV1alpha2(oldWksp, newWksp, req.UserInfo.UID)
	if !allowed {
		return admission.Denied(msg)
	}

	oldCreator, found := oldWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate devworkspace to get it initialized", constants.DevWorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		if newWksp.Labels == nil {
			newWksp.Labels = map[string]string{}
		}
		newWksp.Labels[constants.DevWorkspaceCreatorLabel] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel))
	}

	return admission.Allowed("new workspace has the same devworkspace as old one")
}
