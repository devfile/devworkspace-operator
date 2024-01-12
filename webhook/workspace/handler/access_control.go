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

package handler

import (
	"context"
	"fmt"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// validateUserPermissions validates that the user making the request has permissions to create/update the given workspace.
// In case we're validating a workspace on creation, parameter oldWksp should be set to nil. Returns an error if the user
// cannot perform the requested changes, or if an unexpected error occurs.
// Note: we only perform validation on v1alpha2 DevWorkspaces at the moment, as v1alpha1 DevWorkspaces do not support attributes.
func (h *WebhookHandler) validateUserPermissions(ctx context.Context, req admission.Request, newWksp, oldWksp *dwv2.DevWorkspace) error {
	if !newWksp.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		// Workspace is not requesting anything we need to check RBAC for.
		return nil
	}

	var attributeDecodeErr error
	newSCCAttr := newWksp.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, &attributeDecodeErr)
	if attributeDecodeErr != nil {
		return fmt.Errorf("failed to read %s attribute in DevWorkspace: %s", constants.WorkspaceSCCAttribute, attributeDecodeErr)
	}

	if oldWksp != nil && oldWksp.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		// If we're updating a DevWorkspace, check RBAC only if the relevant attribute is modified to avoid performing too many SARs.
		oldSCCAttr := oldWksp.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, &attributeDecodeErr)
		if attributeDecodeErr != nil {
			return fmt.Errorf("failed to read %s attribute in DevWorkspace: %s", constants.WorkspaceSCCAttribute, attributeDecodeErr)
		}
		if oldSCCAttr == newSCCAttr {
			// RBAC has already been checked for this setting, don't recheck
			return nil
		}
		if oldSCCAttr != newSCCAttr {
			// Don't allow attribute to be changed once it is set, otherwise we can't clean up the SCC when the workspace is deleted.
			return fmt.Errorf("%s attribute cannot be modified after being set -- workspace must be deleted", constants.WorkspaceSCCAttribute)
		}
	}

	if err := h.validateOpenShiftSCC(ctx, req, newSCCAttr); err != nil {
		return err
	}

	return nil
}

func (h *WebhookHandler) validateOpenShiftSCC(ctx context.Context, req admission.Request, scc string) error {
	if !infrastructure.IsOpenShift() {
		// We can only check the appropriate capability on OpenShift currently (via securitycontextconstraints.security.openshift.io)
		// so forbid using this attribute
		return fmt.Errorf("specifying additional SCCs is only permitted on OpenShift")
	}

	if scc == "" {
		return fmt.Errorf("empty value for attribute %s is invalid", constants.WorkspaceSCCAttribute)
	}

	sar := &v1.LocalSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
		},
		Spec: v1.SubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: req.Namespace,
				Verb:      "use",
				Group:     "security.openshift.io",
				Resource:  "securitycontextconstraints",
				Name:      scc,
			},
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			UID:    req.UserInfo.UID,
		},
	}

	err := h.Client.Create(ctx, sar)
	if err != nil {
		return fmt.Errorf("failed to create subjectaccessreview for request: %w", err)
	}

	if !sar.Status.Allowed {
		if sar.Status.Reason != "" {
			return fmt.Errorf("user is not permitted to use the %s SCC: %s", scc, sar.Status.Reason)
		}
		return fmt.Errorf("user is not permitted use the %s SCC", scc)
	}

	return nil
}
