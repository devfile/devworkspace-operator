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
	"encoding/json"
	"fmt"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WorkspaceResourcesMutator checks that every:
// - workspace has creator annotation specified and it's not modified
// - workspace-related deployment, pod has unmodified workspace-id label and creator annotation
type WorkspaceResourcesMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

// WorkspaceResourcesMutator verify if operation is a valid from Workspace controller perspective
func (m *WorkspaceResourcesMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case v1beta1.Create:
		{
			switch req.Kind {
			case V1alpha1WorkspaceKind:
				return m.mutateWorkspaceOnCreate(ctx, req)
			case V1PodKind:
				return m.mutatePodOnCreate(ctx, req)
			case AppsV1DeploymentKind:
				return m.mutateDeploymentOnCreate(ctx, req)
			}
		}
	case v1beta1.Update:
		{
			switch req.Kind {
			case V1alpha1WorkspaceKind:
				return m.mutateWorkspaceOnUpdate(ctx, req)
			case V1PodKind:
				return m.mutatePodOnUpdate(ctx, req)
			case AppsV1DeploymentKind:
				return m.mutateDeploymentOnUpdate(ctx, req)
			}
		}
	}
	// Do not allow operation if the corresponding handler is not found
	// It indicates that the webhooks configuration is not a valid or incompatible with this version of controller
	return admission.Denied(fmt.Sprintf("This admission is not supposed to handle %s operation for %s. Let administrator to know about this issue", req.Operation, req.Kind))
}

func (m *WorkspaceResourcesMutator) parse(req admission.Request, intoOld runtime.Object, intoNew runtime.Object) error {
	err := m.decoder.Decode(req, intoNew)
	if err != nil {
		return err
	}

	err = m.decoder.DecodeRaw(req.OldObject, intoOld)
	if err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceResourcesMutator) returnPatched(req admission.Request, patched runtime.Object) admission.Response {
	marshaledObject, err := json.Marshal(patched)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObject)
}

// WorkspaceMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *WorkspaceResourcesMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// WorkspaceMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *WorkspaceResourcesMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
