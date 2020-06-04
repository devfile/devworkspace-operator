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
	"fmt"
	"net/http"

	"github.com/google/go-cmp/cmp/cmpopts"
	devworkspace "github.com/devfile/kubernetes-api/pkg/apis/workspaces/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var V1alpha1WorkspaceKind = metav1.GroupVersionKind{Kind: "DevWorkspace", Group: "devfile.io", Version: "v1alpha1"}

// StopStartDiffOption is comparing options that should be used to check if there is no other changes except changing started
var StopStartDiffOption = []cmp.Option{
	// field managed by cluster and should be ignored while comparing
	cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ManagedFields"),
	cmpopts.IgnoreFields(devworkspace.DevWorkspaceSpec{}, "Started"),
}

func (h *WebhookHandler) MutateWorkspaceOnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &devworkspace.DevWorkspace{}

	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if wksp.Labels == nil {
		wksp.Labels = map[string]string{}
	}
	wksp.Labels[config.WorkspaceCreatorLabel] = req.UserInfo.UID
	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceOnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &devworkspace.DevWorkspace{}
	oldWksp := &devworkspace.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	immutable := oldWksp.Annotations[config.WorkspaceImmutableAnnotation]
	if immutable == "true" {
		return h.handleImmutableWorkspace(oldWksp, newWksp)
	}

	oldCreator, found := oldWksp.Labels[config.WorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate workspace to get it initialized", config.WorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[config.WorkspaceCreatorLabel]
	if !found {
		newWksp.Labels[config.WorkspaceCreatorLabel] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once workspace is created and is immutable", config.WorkspaceCreatorLabel))
	}

	return admission.Allowed("new workspace has the same workspace as old one")
}

func (h *WebhookHandler) handleImmutableWorkspace(oldWksp, newWksp *devworkspace.DevWorkspace) admission.Response {
	if cmp.Equal(oldWksp, newWksp, StopStartDiffOption[:]...) {
		return admission.Allowed("immutable workspace is started/stopped")
	}
	return admission.Denied(fmt.Sprintf("workspace '%s' is immutable. To make modifications it must be deleted and recreated", oldWksp.Name))
}
