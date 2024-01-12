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

// parse decodes the old and new objects in an admission request. Returns an error if req.OldObject is empty (the field
// is filled only if the request is an UPDATE or DELETE.
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
