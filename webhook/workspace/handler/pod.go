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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutatePodOnCreate(_ context.Context, req admission.Request) admission.Response {
	p := &corev1.Pod{}

	err := h.Decoder.Decode(req, p)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	err = h.mutateMetadataOnCreate(&p.ObjectMeta)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return admission.Allowed("The object is valid")
}

func (h *WebhookHandler) MutatePodOnUpdate(_ context.Context, req admission.Request) admission.Response {
	oldP := &corev1.Pod{}
	newP := &corev1.Pod{}

	err := h.parse(req, oldP, newP)
	if err != nil {
		return admission.Denied(err.Error())
	}

	if ok, msg := h.handleImmutableObj(oldP, newP, req.UserInfo.UID); !ok {
		return admission.Denied(msg)
	}

	if ok, msg := h.handleImmutablePod(oldP, newP, req.UserInfo.UID); !ok {
		return admission.Denied(msg)
	}

	patched, err := h.mutateMetadataOnUpdate(&oldP.ObjectMeta, &newP.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	if patched {
		return h.returnPatched(req, newP)
	}

	return admission.Allowed("The object is valid")
}
