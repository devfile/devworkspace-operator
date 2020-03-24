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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var AppsV1DeploymentKind = metav1.GroupVersionKind{Kind: "Deployment", Group: "apps", Version: "v1"}

func (m *WorkspaceResourcesMutator) mutateDeploymentOnCreate(_ context.Context, req admission.Request) admission.Response {
	d := &appsv1.Deployment{}

	err := m.decoder.Decode(req, d)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = m.mutateMetadataOnCreate(&d.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	err = m.mutateMetadataOnCreate(&d.Spec.Template.ObjectMeta)
	if err != nil {
		return admission.Denied(".spec.template.metadata validation failed: " + err.Error())
	}

	return admission.Allowed("The deployment is valid")
}

func (m *WorkspaceResourcesMutator) mutateDeploymentOnUpdate(_ context.Context, req admission.Request) admission.Response {
	oldD := &appsv1.Deployment{}
	newD := &appsv1.Deployment{}

	err := m.parse(req, oldD, newD)
	if err != nil {
		return admission.Denied(err.Error())
	}

	patchedMeta, err := m.mutateMetadataOnUpdate(&oldD.ObjectMeta, &newD.ObjectMeta)
	if err != nil {
		return admission.Denied(".metadata validation failed: " + err.Error())
	}

	patchedTemplate, err := m.mutateMetadataOnUpdate(&oldD.Spec.Template.ObjectMeta, &newD.Spec.Template.ObjectMeta)
	if err != nil {
		return admission.Denied(".spec.template.metadata validation failed: " + err.Error())
	}

	if patchedMeta || patchedTemplate {
		return m.returnPatched(req, newD)
	}

	return admission.Allowed("The deployment is valid")
}
