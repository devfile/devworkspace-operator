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
	"net/http"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var V1PodExecOptionKind = metav1.GroupVersionKind{Kind: "PodExecOptions", Group: "", Version: "v1"}

func (h *WebhookHandler) ValidateExecOnConnect(ctx context.Context, req admission.Request) admission.Response {
	p := corev1.Pod{}
	err := h.Client.Get(ctx, types.NamespacedName{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, &p)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return admission.Allowed("Not restricted-access DevWorkspace pod")
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	_, ok := p.Labels[constants.DevWorkspaceIDLabel]
	if !ok {
		return admission.Allowed("Not a devworkspace related pod")
	}

	creator, ok := p.Labels[constants.DevWorkspaceCreatorLabel]
	if !ok {
		return admission.Denied("The workspace info is missing in the devworkspace-related pod")
	}

	if p.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation] == "true" &&
		creator != req.UserInfo.UID {
		return admission.Denied("The only devworkspace creator has exec access")
	}

	return admission.Allowed("The current user and devworkspace are matched")
}
