//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
