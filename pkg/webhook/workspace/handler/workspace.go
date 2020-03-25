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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var V1alpha1WorkspaceKind = metav1.GroupVersionKind{Kind: "Workspace", Group: "workspace.che.eclipse.org", Version: "v1alpha1"}

func (h *WebhookHandler) MutateWorkspaceOnCreate(_ context.Context, req admission.Request) admission.Response {
	wksp := &v1alpha1.Workspace{}

	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if wksp.Annotations == nil {
		wksp.Annotations = map[string]string{}
	}
	wksp.Annotations[model.WorkspaceCreatorAnnotation] = req.UserInfo.UID
	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceOnUpdate(_ context.Context, req admission.Request) admission.Response {
	newWksp := &v1alpha1.Workspace{}
	oldWksp := &v1alpha1.Workspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	oldCreator, found := oldWksp.Annotations[model.WorkspaceCreatorAnnotation]
	if !found {
		log.Info(fmt.Sprintf("WARN: Worskpace %s does not have workspace annotation. It happens only for old "+
			"workspaces or when mutating webhook is not configured properly", newWksp.ObjectMeta.UID))
		newWksp.Annotations[model.WorkspaceCreatorAnnotation] = req.UserInfo.UID
		return h.returnPatched(req, newWksp)
	}

	newCreator, found := newWksp.Annotations[model.WorkspaceCreatorAnnotation]
	if !found {
		newWksp.Annotations[model.WorkspaceCreatorAnnotation] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("annotation '%s' is assigned once workspace is created and is immutable", model.WorkspaceCreatorAnnotation))
	}

	return admission.Allowed("new workspace has the same workspace as old one")
}
