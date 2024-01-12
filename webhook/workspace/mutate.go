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

package workspace

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/webhook/workspace/handler"
	admissionv1 "k8s.io/api/admission/v1"
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
	case admissionv1.Create:
		{
			switch req.Kind {
			case handler.V1alpha1DevWorkspaceKind:
				return m.MutateWorkspaceV1alpha1OnCreate(ctx, req)
			case handler.V1alpha2DevWorkspaceKind:
				return m.MutateWorkspaceV1alpha2OnCreate(ctx, req)
			case handler.V1PodKind:
				return m.MutatePodOnCreate(ctx, req)
			case handler.AppsV1DeploymentKind:
				return m.MutateDeploymentOnCreate(ctx, req)
			case handler.V1ServiceKind, handler.V1IngressKind, handler.V1RouteKind, handler.V1JobKind,
				handler.V1alpha1ComponentKind, handler.V1alpha1DevWorkspaceRoutingKind:

				return m.HandleRestrictedAccessCreate(ctx, req)
			}
		}
	case admissionv1.Update:
		{
			switch req.Kind {
			case handler.V1alpha1DevWorkspaceKind:
				return m.MutateWorkspaceV1alpha1OnUpdate(ctx, req)
			case handler.V1alpha2DevWorkspaceKind:
				return m.MutateWorkspaceV1alpha2OnUpdate(ctx, req)
			case handler.V1PodKind:
				return m.MutatePodOnUpdate(ctx, req)
			case handler.AppsV1DeploymentKind:
				return m.MutateDeploymentOnUpdate(ctx, req)
			case handler.V1ServiceKind, handler.V1IngressKind, handler.V1RouteKind, handler.V1JobKind,
				handler.V1alpha1ComponentKind, handler.V1alpha1DevWorkspaceRoutingKind:

				return m.HandleRestrictedAccessUpdate(ctx, req)
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
