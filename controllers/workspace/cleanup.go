// Copyright (c) 2019-2021 Red Hat, Inc.
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

package controllers

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *DevWorkspaceReconciler) deleteWorkspaceOwnedObjects(ctx context.Context, workspace *dw.DevWorkspace) (requeue bool, err error) {
	idLabelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.DevWorkspaceIDLabel, workspace.Status.DevWorkspaceId))
	if err != nil {
		return false, err
	}
	listOptions := &client.ListOptions{
		Namespace:     workspace.Namespace,
		LabelSelector: idLabelSelector,
	}

	deleteObj := func(obj client.Object) error {
		ownerrefs := obj.GetOwnerReferences()
		if len(ownerrefs) != 1 {
			// Either not owned by the DevWorkspace or has multiple owners
			return nil
		}
		if ownerrefs[0].UID != workspace.UID {
			// Not owned by DevWorkspace; shouldn't delete
			return nil
		}

		err := r.Client.Delete(ctx, obj)
		if err != nil && k8sErrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	didDelete := false
	deploymentList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deploymentList, listOptions); err != nil {
		return false, err
	}
	for _, deploy := range deploymentList.Items {
		didDelete = true
		if err := deleteObj(&deploy); err != nil {
			return false, err
		}
	}

	cmList := &corev1.ConfigMapList{}
	if err := r.Client.List(ctx, cmList, listOptions); err != nil {
		return false, err
	}
	for _, cm := range cmList.Items {
		didDelete = true
		if err := deleteObj(&cm); err != nil {
			return false, err
		}
	}

	secretList := &corev1.SecretList{}
	if err := r.Client.List(ctx, secretList, listOptions); err != nil {
		return false, err
	}
	for _, secret := range secretList.Items {
		didDelete = true
		if err := deleteObj(&secret); err != nil {
			return false, err
		}
	}

	routingList := &controllerv1alpha1.DevWorkspaceRoutingList{}
	if err := r.Client.List(ctx, routingList, listOptions); err != nil {
		return false, err
	}
	for _, routing := range routingList.Items {
		didDelete = true
		if err := deleteObj(&routing); err != nil {
			return false, err
		}
	}

	return didDelete, nil
}
