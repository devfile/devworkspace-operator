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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"k8s.io/api/admission/v1beta1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WorkspaceAnnotator annotates Workspaces
type WorkspaceAnnotator struct {
	client  client.Client
	decoder *admission.Decoder
}

// WorkspaceAnnotator adds an creator annotation to every incoming workspaces.
func (a *WorkspaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case v1beta1.Create:
		return a.handleCreate(ctx, req)
	case v1beta1.Update:
		return a.handleUpdate(ctx, req)
	default:
		return admission.Denied(fmt.Sprintf("This admission is not supposed to handle %s operation. Please revise configuration", req.Operation))
	}
}

func (a *WorkspaceAnnotator) handleCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &v1alpha1.Workspace{}

	err := a.decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	log.Info("Provision creator annotation", "workspaceId", wksp.Status.WorkspaceId, "creator", req.UserInfo.UID)

	if wksp.Annotations == nil {
		wksp.Annotations = map[string]string{}
	}
	wksp.Annotations[model.WorkspaceCreatorAnnotation] = req.UserInfo.UID

	marshaledWksp, err := json.Marshal(wksp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledWksp)
}

func (a *WorkspaceAnnotator) handleUpdate(ctx context.Context, req admission.Request) admission.Response {
	newWksp := &v1alpha1.Workspace{}
	oldWksp := &v1alpha1.Workspace{}

	err := a.decoder.Decode(req, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = a.decoder.DecodeRaw(req.OldObject, oldWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	oldCreator, found := oldWksp.Annotations[model.WorkspaceCreatorAnnotation]
	if !found {
		log.Info(fmt.Sprintf("WARN: Worskpace %s does not have creator annotation. It happens only for old "+
			"workspaces or when mutating webhook is not configured properly", newWksp.ObjectMeta.UID))
		return returnPatchedWithUser(req.Object.Raw, newWksp, req.UserInfo.UID)
	}

	newCreator, found := newWksp.Annotations[model.WorkspaceCreatorAnnotation]
	if !found {
		return returnPatchedWithUser(req.Object.Raw, newWksp, oldCreator)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("annotation %s is immutable and must have value: %q", model.WorkspaceCreatorAnnotation, oldCreator))
	}

	return admission.Allowed("new workspace has the same creator as old one")
}

func returnPatchedWithUser(original []byte, patchedWksp *v1alpha1.Workspace, creator string) admission.Response {
	patchedWksp.Annotations[model.WorkspaceCreatorAnnotation] = creator

	marshaledWksp, err := json.Marshal(patchedWksp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(original, marshaledWksp)
}

// WorkspaceAnnotator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (a *WorkspaceAnnotator) InjectClient(c client.Client) error {
	a.client = c
	return nil
}

// WorkspaceAnnotator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *WorkspaceAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
