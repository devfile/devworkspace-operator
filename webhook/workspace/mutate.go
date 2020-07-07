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

// ResourcesMutator checks that every:
// - workspace has creator label specified and it's not modified
// - workspace-related deployment, pod has unmodified workspace-id label and creator label
type ResourcesMutator struct {
	*handler.WebhookHandler
}

func NewResourcesMutator(controllerUID, controllerSAName string) *ResourcesMutator {
	return &ResourcesMutator{&handler.WebhookHandler{ControllerUID: controllerUID, ControllerSAName: controllerSAName}}
}

// ResourcesMutator verify if operation is a valid from Workspace controller perspective
func (m *ResourcesMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case v1beta1.Create:
		{
			switch req.Kind {
			case handler.V1alpha1DevWorkspaceKind:
				return m.MutateWorkspaceOnCreate(ctx, req)
			case handler.V1PodKind:
				return m.MutatePodOnCreate(ctx, req)
			case handler.AppsV1DeploymentKind:
				return m.MutateDeploymentOnCreate(ctx, req)
			case handler.V1ServiceKind, handler.V1beta1IngressKind, handler.V1RouteKind,
				handler.V1alpha1ComponentKind, handler.V1alpha1WorkspaceRoutingKind:

				return m.HandleImmutableCreate(ctx, req)
			}
		}
	case v1beta1.Update:
		{
			switch req.Kind {
			case handler.V1alpha1DevWorkspaceKind:
				return m.MutateWorkspaceOnUpdate(ctx, req)
			case handler.V1PodKind:
				return m.MutatePodOnUpdate(ctx, req)
			case handler.AppsV1DeploymentKind:
				return m.MutateDeploymentOnUpdate(ctx, req)
			case handler.V1ServiceKind, handler.V1beta1IngressKind, handler.V1RouteKind,
				handler.V1alpha1ComponentKind, handler.V1alpha1WorkspaceRoutingKind:

				return m.HandleImmutableMutate(ctx, req)
			}
		}
	}
	// Do not allow operation if the corresponding handler is not found
	// It indicates that the webhooks configuration is not a valid or incompatible with this version of controller
	return admission.Denied(fmt.Sprintf("This admission controller is not designed to handle %s operation for %s. Notify an administrator about this issue", req.Operation, req.Kind))
}

// WorkspaceMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *ResourcesMutator) InjectClient(c client.Client) error {
	m.Client = c
	return nil
}

// WorkspaceMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *ResourcesMutator) InjectDecoder(d *admission.Decoder) error {
	m.Decoder = d
	return nil
}
