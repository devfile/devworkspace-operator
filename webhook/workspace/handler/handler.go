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
//

package handler

import (
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type WebhookHandler struct {
	ControllerUID    string
	ControllerSAName string
	Client           client.Client
	Decoder          *admission.Decoder
}

func (h *WebhookHandler) parse(req admission.Request, intoOld runtime.Object, intoNew runtime.Object) error {
	err := h.Decoder.Decode(req, intoNew)
	if err != nil {
		return err
	}

	err = h.Decoder.DecodeRaw(req.OldObject, intoOld)
	if err != nil {
		return err
	}
	return nil
}

func (h *WebhookHandler) returnPatched(req admission.Request, patched runtime.Object) admission.Response {
	marshaledObject, err := json.Marshal(patched)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObject)
}
