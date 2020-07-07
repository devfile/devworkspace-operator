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
package workspace

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/webhook/workspace/handler"
	"k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ResourcesValidator validates execs process all exec requests and:
// if related pod DOES NOT have workspace_id label - just skip it
// if related pod DOES have workspace_id label - make sure that exec is requested by workspace creator
type ResourcesValidator struct {
	*handler.WebhookHandler
}

func NewResourcesValidator(controllerUID, controllerSAName string) *ResourcesValidator {
	return &ResourcesValidator{&handler.WebhookHandler{ControllerUID: controllerUID, ControllerSAName: controllerSAName}}
}

func (v *ResourcesValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Kind == handler.V1PodExecOptionKind && req.Operation == v1beta1.Connect {
		return v.ValidateExecOnConnect(ctx, req)
	}
	// Do not allow operation if the corresponding handler is not found
	// It indicates that the webhooks configuration is not a valid or incompatible with this version of controller
	return admission.Denied(fmt.Sprintf("This admission controller is not designed to handle %s operation for %s. Notify an administrator about this issue", req.Operation, req.Kind))
}

// WorkspaceMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (v *ResourcesValidator) InjectClient(c client.Client) error {
	v.Client = c
	return nil
}

// WorkspaceMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *ResourcesValidator) InjectDecoder(d *admission.Decoder) error {
	v.Decoder = d
	return nil
}
