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
	"context"
	"fmt"
	"reflect"

	"github.com/che-incubator/che-workspace-operator/pkg/config"
	devworkspace "github.com/devfile/kubernetes-api/pkg/apis/workspaces/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) HandleImmutableMutate(_ context.Context, req admission.Request) admission.Response {
	oldObj := &unstructured.Unstructured{}
	newObj := &unstructured.Unstructured{}
	err := h.parse(req, oldObj, newObj)
	if err != nil {
		return admission.Denied(err.Error())
	}
	allowed, msg := h.handleImmutableObj(oldObj, newObj, req.UserInfo.UID)
	if allowed {
		return admission.Allowed(msg)
	}
	return admission.Denied(msg)
}

func (h *WebhookHandler) HandleImmutableCreate(_ context.Context, req admission.Request) admission.Response {
	if req.UserInfo.UID != h.ControllerUID {
		return admission.Denied("Only the workspace controller can create workspace objects.")
	}
	return admission.Allowed("Object created by workspace controller service account.")
}

func (h *WebhookHandler) handleImmutableWorkspace(oldWksp, newWksp *devworkspace.DevWorkspace, uid string) (allowed bool, msg string) {
	creatorUID := oldWksp.Labels[config.WorkspaceCreatorLabel]
	if uid == creatorUID || uid == h.ControllerUID {
		return true, "immutable workspace is updated by owner or controller"
	}
	if cmp.Equal(oldWksp, newWksp, StopStartDiffOption[:]...) {
		return true, "immutable workspace is started/stopped"
	}
	return false, fmt.Sprintf("workspace '%s' is immutable and can only be modified by its creator.", oldWksp.Name)
}

func (h *WebhookHandler) handleImmutableObj(oldObj, newObj runtime.Object, uid string) (allowed bool, msg string) {
	if uid == h.ControllerUID {
		return true, ""
	}
	if reflect.DeepEqual(oldObj, newObj) {
		return true, ""
	}
	return false, "object is owned by workspace and cannot be updated."
}
