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

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const serviceCAUsername = "system:serviceaccount:openshift-service-ca:service-ca"

// ImmutableWorkspaceDiffOptions is comparing options that should be used to check if there is no other changes except changing started
var ImmutableWorkspaceDiffOptions = []cmp.Option{
	// field managed by cluster and should be ignored while comparing
	cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ManagedFields", "Finalizers", "DeletionTimestamp"),
	cmpopts.IgnoreFields(devworkspace.DevWorkspaceSpec{}, "Started"),
}

func (h *WebhookHandler) HandleImmutableMutate(_ context.Context, req admission.Request) admission.Response {
	oldObj := &unstructured.Unstructured{}
	newObj := &unstructured.Unstructured{}
	err := h.parse(req, oldObj, newObj)
	if err != nil {
		return admission.Denied(err.Error())
	}
	var allowed bool
	var msg string
	if req.Kind == V1RouteKind {
		allowed, msg = h.handleImmutableRoute(oldObj, newObj, req.UserInfo.Username)
	} else if req.Kind == V1ServiceKind {
		allowed, msg = h.handleImmutableService(oldObj, newObj, req.UserInfo.UID, req.UserInfo.Username)
	} else {
		allowed, msg = h.handleImmutableObj(oldObj, newObj, req.UserInfo.UID)
	}
	if allowed {
		return admission.Allowed(msg)
	}

	log.Info(fmt.Sprintf("Denied request to %s '%s' %s by user %s: %s", req.Operation, req.Name, req.Kind.Kind, req.UserInfo.Username, msg))
	return admission.Denied(msg)
}

func (h *WebhookHandler) HandleImmutableCreate(_ context.Context, req admission.Request) admission.Response {
	if req.Kind == V1RouteKind && req.UserInfo.Username == h.ControllerSAName {
		// Have to handle this case separately since req.UserInfo.UID is empty for Route objects
		// ref: https://github.com/eclipse/che/issues/17114
		return admission.Allowed("Object created by workspace controller service account.")
	}
	if req.UserInfo.UID == h.ControllerUID {
		return admission.Allowed("Object created by workspace controller service account.")
	}
	return admission.Denied("Only the workspace controller can create workspace objects.")
}

func (h *WebhookHandler) handleImmutableWorkspace(oldWksp, newWksp *devworkspace.DevWorkspace, uid string) (allowed bool, msg string) {
	creatorUID := oldWksp.Labels[config.WorkspaceCreatorLabel]
	if uid == creatorUID || uid == h.ControllerUID {
		return true, "immutable workspace is updated by owner or controller"
	}
	if cmp.Equal(oldWksp, newWksp, ImmutableWorkspaceDiffOptions[:]...) {
		return true, "immutable workspace is started/stopped"
	}
	log.Info(fmt.Sprintf("Denied request on workspace resource by user %s", uid))
	return false, fmt.Sprintf("workspace '%s' is immutable and can only be modified by its creator.", oldWksp.Name)
}

func (h *WebhookHandler) handleImmutableObj(oldObj, newObj runtime.Object, uid string) (allowed bool, msg string) {
	if uid == h.ControllerUID {
		return true, ""
	}
	if changePermitted(oldObj, newObj) {
		return true, ""
	}
	return false, "object is owned by workspace and cannot be updated."
}

func (h *WebhookHandler) handleImmutableRoute(oldObj, newObj runtime.Object, username string) (allowed bool, msg string) {
	if username == h.ControllerSAName {
		return true, ""
	}
	if changePermitted(oldObj, newObj) {
		return true, ""
	}
	return false, "object is owned by workspace and cannot be updated."
}

func (h *WebhookHandler) handleImmutableService(oldObj, newObj runtime.Object, uid, username string) (allowed bool, msg string) {
	// Special case: secure services are updated by the service-ca serviceaccount once a secret is created to contain
	// tls key and cert.
	if uid == h.ControllerUID || username == serviceCAUsername {
		return true, ""
	}
	if changePermitted(oldObj, newObj) {
		return true, ""
	}
	return false, "object is owned by workspace and cannot be updated."
}

func changePermitted(oldObj, newObj runtime.Object) bool {
	oldCopy := oldObj.DeepCopyObject()
	newCopy := newObj.DeepCopyObject()
	oldMeta, ok := oldCopy.(metav1.Object)
	if !ok {
		log.Error(fmt.Errorf("Object %s is not a valid k8s object: does not have metadata", oldObj.GetObjectKind()), "Failed to compare objects")
		return false
	}
	newMeta, ok := newCopy.(metav1.Object)
	if !ok {
		log.Error(fmt.Errorf("Object %s is not a valid k8s object: does not have metadata", newObj.GetObjectKind()), "Failed to compare objects")
		return false
	}
	newMeta.SetFinalizers(oldMeta.GetFinalizers())
	newMeta.SetDeletionTimestamp(oldMeta.GetDeletionTimestamp())
	return reflect.DeepEqual(oldMeta, newMeta)
}
