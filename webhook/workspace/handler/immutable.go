//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	devworkspacev1alpha1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	devworkspacev1alpha2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const serviceCAUsername = "system:serviceaccount:openshift-service-ca:service-ca"

// RestrictedAccessDiffOptions is comparing options that should be used to check for changes to a devworkspace. Changing
// the .spec.started field is permitted. Note: Does not check metadata; use checkRestrictedWorkspaceMetadata for this.
var RestrictedAccessDiffOptions = []cmp.Option{
	// field managed by cluster and should be ignored while comparing
	cmpopts.IgnoreTypes(metav1.ObjectMeta{}),
	cmpopts.IgnoreFields(devworkspacev1alpha1.DevWorkspaceSpec{}, "Started"),
	cmpopts.IgnoreFields(devworkspacev1alpha2.DevWorkspaceSpec{}, "Started"),
}

func (h *WebhookHandler) HandleRestrictedAccessUpdate(_ context.Context, req admission.Request) admission.Response {
	isRestricted, err := h.checkRestrictedAccessAnnotation(req)
	if err != nil {
		return admission.Denied(err.Error())
	}
	if !isRestricted {
		return admission.Allowed("Workspace does not have restricted-access annotation")
	}

	oldObj := &unstructured.Unstructured{}
	newObj := &unstructured.Unstructured{}
	err = h.parse(req, oldObj, newObj)
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

func (h *WebhookHandler) HandleRestrictedAccessCreate(_ context.Context, req admission.Request) admission.Response {
	isRestricted, err := h.checkRestrictedAccessAnnotation(req)
	if err != nil {
		return admission.Denied(err.Error())
	}
	if !isRestricted {
		return admission.Allowed("Workspace does not have restricted-access annotation")
	}

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

func (h *WebhookHandler) checkRestrictedAccessWorkspaceV1alpha1(oldWksp, newWksp *devworkspacev1alpha1.DevWorkspace, uid string) (allowed bool, msg string) {
	if oldWksp.Annotations[constants.WorkspaceRestrictedAccessAnnotation] != "true" {
		return true, "workspace does not have restricted access configured"
	}
	creatorUID := oldWksp.Labels[constants.WorkspaceCreatorLabel]
	if uid == creatorUID || uid == h.ControllerUID {
		return true, "workspace with restricted-access is updated by owner or controller"
	}
	if !cmp.Equal(oldWksp, newWksp, RestrictedAccessDiffOptions[:]...) {
		return false, "workspace has restricted-access enabled and can only be modified by its creator."
	}
	return checkRestrictedWorkspaceMetadata(&oldWksp.ObjectMeta, &newWksp.ObjectMeta)
}

func (h *WebhookHandler) checkRestrictedAccessWorkspaceV1alpha2(oldWksp, newWksp *devworkspacev1alpha2.DevWorkspace, uid string) (allowed bool, msg string) {
	if oldWksp.Annotations[constants.WorkspaceRestrictedAccessAnnotation] != "true" {
		return true, "workspace does not have restricted access configured"
	}
	creatorUID := oldWksp.Labels[constants.WorkspaceCreatorLabel]
	if uid == creatorUID || uid == h.ControllerUID {
		return true, "workspace with restricted-access is updated by owner or controller"
	}
	if !cmp.Equal(oldWksp, newWksp, RestrictedAccessDiffOptions[:]...) {
		return false, "workspace has restricted-access enabled and can only be modified by its creator."
	}
	return checkRestrictedWorkspaceMetadata(&oldWksp.ObjectMeta, &newWksp.ObjectMeta)
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

func (h *WebhookHandler) checkRestrictedAccessAnnotation(req admission.Request) (restrictedAccess bool, err error) {
	obj := &unstructured.Unstructured{}
	// If request is UPDATE/DELETE, use old object to check annotation; otherwise, use new object
	if len(req.OldObject.Raw) > 0 {
		err = h.Decoder.DecodeRaw(req.OldObject, obj)
	} else {
		err = h.Decoder.DecodeRaw(req.Object, obj)
	}
	annotations := obj.GetAnnotations()
	return annotations[constants.WorkspaceRestrictedAccessAnnotation] == "true", nil
}

func checkRestrictedWorkspaceMetadata(oldMeta, newMeta *metav1.ObjectMeta) (allowed bool, msg string) {
	if oldMeta.Labels[constants.WorkspaceCreatorLabel] != newMeta.Labels[constants.WorkspaceCreatorLabel] {
		return false, "cannot update controller.devfile.io/creator label"
	}
	if oldMeta.Annotations[constants.WorkspaceRestrictedAccessAnnotation] == "true" &&
		newMeta.Annotations[constants.WorkspaceRestrictedAccessAnnotation] != "true" {
		return false, "cannot disable restricted-access once it is set"
	}
	return true, "permitted change to workspace"
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
	oldLabels, newLabels := oldMeta.GetLabels(), newMeta.GetLabels()
	oldAnnotations, newAnnotations := oldMeta.GetAnnotations(), newMeta.GetAnnotations()
	if oldLabels[constants.WorkspaceCreatorLabel] != newLabels[constants.WorkspaceCreatorLabel] {
		return false
	}
	if oldAnnotations[constants.WorkspaceRestrictedAccessAnnotation] == "true" &&
		newAnnotations[constants.WorkspaceRestrictedAccessAnnotation] != "true" {
		return false
	}
	return true
}
