package handler

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type WebhookHandler struct {
	Client  client.Client
	Decoder *admission.Decoder
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
