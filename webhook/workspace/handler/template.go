// Copyright (c) 2019-2024 Red Hat, Inc.
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

package handler

import (
	"context"
	"fmt"
	"net/http"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceTemplateV1alpha1OnCreate(ctx context.Context, req admission.Request) admission.Response {
	wksp := &dwv1.DevWorkspaceTemplate{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.validateKubernetesObjectPermissionsOnCreate_v1alpha1(ctx, req, &wksp.Spec); err != nil {
		return admission.Denied(err.Error())
	}

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceTemplateV1alpha2OnCreate(ctx context.Context, req admission.Request) admission.Response {
	wksp := &dwv2.DevWorkspaceTemplate{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.validateKubernetesObjectPermissionsOnCreate(ctx, req, &wksp.Spec); err != nil {
		return admission.Denied(err.Error())
	}

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceTemplateV1alpha1OnUpdate(ctx context.Context, req admission.Request) admission.Response {
	newWksp := &dwv1.DevWorkspaceTemplate{}
	oldWksp := &dwv1.DevWorkspaceTemplate{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.validateKubernetesObjectPermissionsOnUpdate_v1alpha1(ctx, req, &newWksp.Spec, &oldWksp.Spec); err != nil {
		return admission.Denied(err.Error())
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

func (h *WebhookHandler) MutateWorkspaceTemplateV1alpha2OnUpdate(ctx context.Context, req admission.Request) admission.Response {
	newWksp := &dwv2.DevWorkspaceTemplate{}
	oldWksp := &dwv2.DevWorkspaceTemplate{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.validateKubernetesObjectPermissionsOnUpdate(ctx, req, &newWksp.Spec, &oldWksp.Spec); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("new workspace has the same devworkspace as old one")
}
