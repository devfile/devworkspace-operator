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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var V1PodKind = metav1.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}

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

	patched, err := h.mutateMetadataOnUpdate(&oldP.ObjectMeta, &newP.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	if patched {
		return h.returnPatched(req, newP)
	}

	return admission.Allowed("The object is valid")
}
