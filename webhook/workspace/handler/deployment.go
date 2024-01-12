//
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
//

package handler

import (
	"context"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateDeploymentOnCreate(_ context.Context, req admission.Request) admission.Response {
	d := &appsv1.Deployment{}

	err := h.Decoder.Decode(req, d)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = h.mutateMetadataOnCreate(&d.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	err = h.mutateMetadataOnCreate(&d.Spec.Template.ObjectMeta)
	if err != nil {
		return admission.Denied(".spec.template.metadata validation failed: " + err.Error())
	}

	return admission.Allowed("The deployment is valid")
}

func (h *WebhookHandler) MutateDeploymentOnUpdate(_ context.Context, req admission.Request) admission.Response {
	oldD := &appsv1.Deployment{}
	newD := &appsv1.Deployment{}

	err := h.parse(req, oldD, newD)
	if err != nil {
		return admission.Denied(err.Error())
	}

	ok, msg := h.handleImmutableObj(oldD, newD, req.UserInfo.UID)
	if !ok {
		return admission.Denied(msg)
	}

	patchedMeta, err := h.mutateMetadataOnUpdate(&oldD.ObjectMeta, &newD.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	patchedTemplate, err := h.mutateMetadataOnUpdate(&oldD.Spec.Template.ObjectMeta, &newD.Spec.Template.ObjectMeta)
	if err != nil {
		return admission.Denied(".spec.template.metadata validation failed: " + err.Error())
	}

	if patchedMeta || patchedTemplate {
		return h.returnPatched(req, newD)
	}

	return admission.Allowed("The deployment is valid")
}
