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
package creator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// CreatorChecker checks that every workspace-related deployment, pod (who has workspace id label) has creator
// annotation and  it's not modified
type CreatorChecker struct {
	client  client.Client
	decoder *admission.Decoder
}

// CreatorChecker verify if upcoming object has creator annotation
func (a *CreatorChecker) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Kind.Kind == "Deployment" || req.Kind.Kind == "Pod" {
		if req.Operation == v1beta1.Create {
			return a.handleUnstructuredCreate(req)
		} else if req.Operation == v1beta1.Update {
			return a.handleUnstructuredUpdate(req)
		}
	}
	return admission.Denied(fmt.Sprintf("This admission is not supposed to handle %s operation for %s. Let administrator to know about this issue", req.Operation, req.Kind))
}

func (a *CreatorChecker) handleUnstructuredCreate(req admission.Request) admission.Response {
	o := &unstructured.Unstructured{}

	err := a.decoder.Decode(req, o)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if o.GetAnnotations() == nil {
		return admission.Denied(fmt.Sprintf("Workspace related %s must have creator annotation specified", req.Kind))
	} else {
		if _, ok := o.GetAnnotations()[model.WorkspaceCreatorAnnotation]; !ok {
			return admission.Denied(fmt.Sprintf("Workspace related %s must have creator annotation specified", req.Kind))
		}
	}

	return admission.Allowed("creator annotation is present")
}

func (a *CreatorChecker) handleUnstructuredUpdate(req admission.Request) admission.Response {
	newObject := &unstructured.Unstructured{}
	oldObject := &unstructured.Unstructured{}

	err := a.decoder.Decode(req, newObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = a.decoder.DecodeRaw(req.OldObject, oldObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	updated, errMsg := a.checkAnnotations(oldObject.GetAnnotations(), newObject.GetAnnotations(), req.Kind.Kind)

	if errMsg != "" {
		return admission.Denied(errMsg)
	}

	if updated != nil {
		return returnAnnotatedUnstructured(req.Object.Raw, newObject, updated)
	}

	return admission.Allowed("new deployment has the same creator as old one")
}

func (a *CreatorChecker) checkAnnotations(old, new map[string]string, kind string) (map[string]string, string) {
	if old == nil {
		return nil, fmt.Sprintf("%s must have '%s' annotation specified. Update Controller and restart your workspace", kind, model.WorkspaceCreatorAnnotation)
	}
	oldCreator, found := old[model.WorkspaceCreatorAnnotation]
	if !found {
		return nil, fmt.Sprintf("%s must have '%s' annotation specified. Update Controller and restart your workspace", kind, model.WorkspaceCreatorAnnotation)
	}

	if new == nil {
		new = map[string]string{}
	}
	newCreator, found := new[model.WorkspaceCreatorAnnotation]
	if !found {
		new[model.WorkspaceCreatorAnnotation] = oldCreator
		return new, ""
	}

	if newCreator != oldCreator {
		return nil, fmt.Sprintf("annotation '%s' is assigned once workspace is created and is immutable", model.WorkspaceCreatorAnnotation)
	}

	return nil, ""
}

func returnAnnotatedUnstructured(original []byte, patchedObject *unstructured.Unstructured, annotations map[string]string) admission.Response {
	patchedObject.SetAnnotations(annotations)
	marshaledObject, err := json.Marshal(patchedObject)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(original, marshaledObject)
}

// WorkspaceAnnotator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (a *CreatorChecker) InjectClient(c client.Client) error {
	a.client = c
	return nil
}

// WorkspaceAnnotator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *CreatorChecker) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
