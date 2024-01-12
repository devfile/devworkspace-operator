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

package handler

import (
	"context"
	"fmt"
	"net/http"

	maputils "github.com/devfile/devworkspace-operator/internal/map"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnCreate(ctx context.Context, req admission.Request) admission.Response {
	wksp := &dwv1.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	wksp.Labels = maputils.Append(wksp.Labels, constants.DevWorkspaceCreatorLabel, req.UserInfo.UID)

	if err := h.validateKubernetesObjectPermissionsOnCreate_v1alpha1(ctx, req, &wksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnCreate(ctx context.Context, req admission.Request) admission.Response {
	wksp := &dwv2.DevWorkspace{}
	err := h.Decoder.Decode(req, wksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	wksp.Labels = maputils.Append(wksp.Labels, constants.DevWorkspaceCreatorLabel, req.UserInfo.UID)

	if err := h.validateUserPermissions(ctx, req, wksp, nil); err != nil {
		return admission.Denied(err.Error())
	}

	if err := h.validateKubernetesObjectPermissionsOnCreate(ctx, req, &wksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	if err := checkMultipleContainerContributionTargets(wksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	if warnings := checkUnsupportedFeatures(wksp.Spec.Template); unsupportedWarningsPresent(warnings) {
		return h.returnPatched(req, wksp).WithWarnings(formatUnsupportedFeaturesWarning(warnings))
	}

	return h.returnPatched(req, wksp)
}

func (h *WebhookHandler) MutateWorkspaceV1alpha1OnUpdate(ctx context.Context, req admission.Request) admission.Response {
	newWksp := &dwv1.DevWorkspace{}
	oldWksp := &dwv1.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if oldWksp.Status.WorkspaceId != "" && newWksp.Status.WorkspaceId != oldWksp.Status.WorkspaceId {
		return admission.Denied("DevWorkspace ID cannot be changed once it is set")
	}

	allowed, msg := h.checkRestrictedAccessWorkspaceV1alpha1(oldWksp, newWksp, req.UserInfo.UID)
	if !allowed {
		return admission.Denied(msg)
	}

	if err := h.validateKubernetesObjectPermissionsOnUpdate_v1alpha1(ctx, req, &newWksp.Spec.Template, &oldWksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	oldCreator, found := oldWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate devworkspace to get it initialized", constants.DevWorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		if newWksp.Labels == nil {
			newWksp.Labels = map[string]string{}
		}
		newWksp.Labels[constants.DevWorkspaceCreatorLabel] = oldCreator
		return h.returnPatched(req, newWksp)
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel))
	}

	return admission.Allowed("new devworkspace has the same devworkspace creator as old one")
}

func (h *WebhookHandler) MutateWorkspaceV1alpha2OnUpdate(ctx context.Context, req admission.Request) admission.Response {
	newWksp := &dwv2.DevWorkspace{}
	oldWksp := &dwv2.DevWorkspace{}
	err := h.parse(req, oldWksp, newWksp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if oldWksp.Status.DevWorkspaceId != "" && newWksp.Status.DevWorkspaceId != oldWksp.Status.DevWorkspaceId {
		return admission.Denied("DevWorkspace ID cannot be changed once it is set")
	}

	warnings := ""
	addedUnsupportedFeatures := checkForAddedUnsupportedFeatures(oldWksp, newWksp)
	if unsupportedWarningsPresent(addedUnsupportedFeatures) {
		warnings = formatUnsupportedFeaturesWarning(addedUnsupportedFeatures)
	}

	// TODO: re-enable webhooks for storageClass once handling is improved.
	// oldStorageType := oldWksp.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
	// newStorageType := newWksp.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)

	// // Prevent switching storage type when it could risk orphaning data in a PVC (e.g. switching from common to ephemeral)
	// if oldStorageType != newStorageType {
	// 	switch {
	// 	case oldStorageType == constants.EphemeralStorageClassType:
	// 		// Allow switching from ephemeral to a persistent storage type
	// 		break
	// 	case oldStorageType == "" && (newStorageType == constants.CommonStorageClassType || newStorageType == constants.PerUserStorageClassType):
	// 		// Allow switching to per-user or common persistent storage type if the oldStorageType is empty (if empty, the common / per-user PVC strategy is used by design)
	// 		break
	// 	case newStorageType == "" && (oldStorageType == constants.CommonStorageClassType || oldStorageType == constants.PerUserStorageClassType):
	// 		// Allow removing storage type attribute if the oldStorageType is per-user or common (if empty, the common / per-user PVC strategy is used by design)
	// 		break
	// 	case (oldStorageType == constants.CommonStorageClassType && newStorageType == constants.PerUserStorageClassType) || (oldStorageType == constants.PerUserStorageClassType && newStorageType == constants.CommonStorageClassType):
	// 		// Allow switching between common and per-user persistent storage type for legacy compatibility
	// 		break
	// 	case !hasFinalizer(oldWksp, constants.StorageCleanupFinalizer) && !hasFinalizer(newWksp, constants.StorageCleanupFinalizer):
	// 		// If finalizer is not set, the workspace does not use storage yet and so can safely switch (e.g. a workspace was created
	// 		// with `started: false` and then edited)
	// 		break
	// 	default:
	// 		return admission.Denied("DevWorkspace storage-type attribute cannot be changed once the workspace has been created.")
	// 	}
	// }

	allowed, msg := h.checkRestrictedAccessWorkspaceV1alpha2(oldWksp, newWksp, req.UserInfo.UID)
	if !allowed {
		return admission.Denied(msg)
	}

	if err := h.validateUserPermissions(ctx, req, newWksp, oldWksp); err != nil {
		return admission.Denied(err.Error())
	}

	if err := h.validateKubernetesObjectPermissionsOnUpdate(ctx, req, &newWksp.Spec.Template, &oldWksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	if err := checkMultipleContainerContributionTargets(newWksp.Spec.Template); err != nil {
		return admission.Denied(err.Error())
	}

	oldCreator, found := oldWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		return admission.Denied(fmt.Sprintf("label '%s' is missing. Please recreate devworkspace to get it initialized", constants.DevWorkspaceCreatorLabel))
	}

	newCreator, found := newWksp.Labels[constants.DevWorkspaceCreatorLabel]
	if !found {
		if newWksp.Labels == nil {
			newWksp.Labels = map[string]string{}
		}
		newWksp.Labels[constants.DevWorkspaceCreatorLabel] = oldCreator
		response := h.returnPatched(req, newWksp)
		if warnings != "" {
			return response.WithWarnings(warnings)
		}
		return response
	}

	if newCreator != oldCreator {
		return admission.Denied(fmt.Sprintf("label '%s' is assigned once devworkspace is created and is immutable", constants.DevWorkspaceCreatorLabel))
	}

	if warnings != "" {
		return admission.Allowed("").WithWarnings(warnings)
	}
	return admission.Allowed("new workspace has the same devworkspace as old one")
}

func hasFinalizer(obj client.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}
