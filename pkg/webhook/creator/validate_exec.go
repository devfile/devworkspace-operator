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
package creator

import (
	"context"
	"fmt"
	"github.com/che-incubator/che-workspace-operator/internal/controller"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WorkspaceExecValidator validates execs process all exec requests and:
// if related pod DOES NOT have workspace_id label - just skip it
// if related pod DOES have workspace_id label - make sure that exec is requested by workspace creator
type WorkspaceExecValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

func (v *WorkspaceExecValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Resource.Resource == "pods" && req.Operation == v1beta1.Connect {
		return v.handleCreateExec(ctx, req)
	}
	return admission.Denied(fmt.Sprintf("This admission is not supposed to handle %s operation for %s. Let administrator to know about this issue", req.Operation, req.Kind))
}

func (v *WorkspaceExecValidator) handleCreateExec(ctx context.Context, req admission.Request) admission.Response {
	c, err := controller.CreateClient()
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	p := v1.Pod{}
	err = c.Get(ctx, types.NamespacedName{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, &p)

	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if p.Annotations == nil {
		p.Annotations = map[string]string{}
	}

	_, ok := p.Labels[model.WorkspaceIDLabel]
	if !ok {
		return admission.Allowed("It's not workspace related pod")
	}

	creator, ok := p.Annotations[model.WorkspaceCreatorAnnotation]
	if !ok {
		return admission.Denied("The creator info is missing in the workspace-related pod")
	}

	if creator != req.UserInfo.UID {
		return admission.Denied("The only workspace creator has exec access")
	}

	return admission.Allowed("The current user and creator are matched")
}

// WorkspaceAnnotator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (v *WorkspaceExecValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// WorkspaceAnnotator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *WorkspaceExecValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
