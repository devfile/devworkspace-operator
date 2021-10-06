//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &dwv2.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	wksp.Labels = maputils.Append(wksp.Labels, constants.DevWorkspaceCreatorLabel, req.UserInfo.UID)

	return h.returnPatched(req, wksp)
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
