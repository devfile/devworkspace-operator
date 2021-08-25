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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	V1alpha1DevWorkspaceKind        = metav1.GroupVersionKind{Kind: "DevWorkspace", Group: "workspace.devfile.io", Version: "v1alpha1"}
	V1alpha2DevWorkspaceKind        = metav1.GroupVersionKind{Kind: "DevWorkspace", Group: "workspace.devfile.io", Version: "v1alpha2"}
	V1alpha1DevWorkspaceRoutingKind = metav1.GroupVersionKind{Kind: "DevWorkspaceRouting", Group: "controller.devfile.io", Version: "v1alpha1"}
	V1alpha1ComponentKind           = metav1.GroupVersionKind{Kind: "Component", Group: "controller.devfile.io", Version: "v1alpha1"}

	AppsV1DeploymentKind = metav1.GroupVersionKind{Kind: "Deployment", Group: "apps", Version: "v1"}
	V1PodKind            = metav1.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}
	V1ServiceKind        = metav1.GroupVersionKind{Kind: "Service", Group: "", Version: "v1"}
	V1IngressKind        = metav1.GroupVersionKind{Kind: "Ingress", Group: "networking.k8s.io", Version: "v1"}
	V1JobKind            = metav1.GroupVersionKind{Kind: "Job", Group: "batch", Version: "v1"}
	V1RouteKind          = metav1.GroupVersionKind{Kind: "Route", Group: "route.openshift.io", Version: "v1"}
)
