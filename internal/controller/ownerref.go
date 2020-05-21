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
package controller

import (
	"context"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var Log = logf.Log.WithName("ownerref")

//FindControllerOwner returns OwnerReferent that owns controller process
//it starts searching from the current pod and then resolves owners recursively
//until object without owner is not found
func FindControllerOwner(ctx context.Context, client crclient.Client) (*metav1.OwnerReference, error) {
	ns, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	// Get current Pod the operator is running in
	pod, err := k8sutil.GetPod(ctx, client, ns)
	if err != nil {
		return nil, err
	}
	podOwnerRefs := metav1.NewControllerRef(pod, pod.GroupVersionKind())
	// Get Owner that the Pod belongs to
	ownerRef := metav1.GetControllerOf(pod)
	finalOwnerRef, err := findFinalOwnerRef(ctx, client, ns, ownerRef)
	if err != nil {
		return nil, err
	}
	if finalOwnerRef != nil {
		return finalOwnerRef, nil
	}

	return podOwnerRefs, nil
}

// findFinalOwnerRef tries to locate the final controller/owner based on the owner reference provided.
func findFinalOwnerRef(ctx context.Context, client crclient.Client, ns string, ownerRef *metav1.OwnerReference) (*metav1.OwnerReference, error) {
	if ownerRef == nil {
		return nil, nil
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ownerRef.APIVersion)
	obj.SetKind(ownerRef.Kind)
	err := client.Get(ctx, types.NamespacedName{Namespace: ns, Name: ownerRef.Name}, obj)
	if err != nil {
		return nil, err
	}
	newOwnerRef := metav1.GetControllerOf(obj)
	if newOwnerRef != nil {
		return findFinalOwnerRef(ctx, client, ns, newOwnerRef)
	}

	Log.V(1).Info("Pods owner found", "Kind", ownerRef.Kind, "Name", ownerRef.Name, "Namespace", ns)
	return ownerRef, nil
}

// FindControllerDeployment gets the deployment of the deployed controller
func FindControllerDeployment(ctx context.Context, client crclient.Client) (*appsv1.Deployment, error) {
	ns, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	// Get current Pod the operator is running in
	pod, err := k8sutil.GetPod(ctx, client, ns)
	if err != nil {
		return nil, err
	}

	deployment, err := findDeployment(ctx, client, ns, pod)
	if err != nil {
		return nil, err
	}
	if deployment != nil {
		return deployment, nil
	}

	// Default to returning Pod as the Owner
	return nil, nil
}

// findDeployment tries to locate the final controller/owner based on the owner reference provided.
func findDeployment(ctx context.Context, client crclient.Client, ns string, metaObj metav1.Object) (*appsv1.Deployment, error) {
	ownerRef := metav1.GetControllerOf(metaObj)

	if ownerRef == nil {
		return nil, nil
	}

	if ownerRef.Kind == "Deployment" {
		d := &appsv1.Deployment{}
		err := client.Get(ctx, types.NamespacedName{Namespace: ns, Name: ownerRef.Name}, d)
		d.APIVersion = ownerRef.APIVersion
		d.Kind = ownerRef.Kind
		if err != nil {
			return nil, err
		}
		return d, nil
	}
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ownerRef.APIVersion)
	obj.SetKind(ownerRef.Kind)
	err := client.Get(ctx, types.NamespacedName{Namespace: ns, Name: ownerRef.Name}, obj)
	if err != nil {
		return nil, err
	}

	return findDeployment(ctx, client, ns, obj)
}
