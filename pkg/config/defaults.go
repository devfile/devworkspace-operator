//
// Copyright (c) 2019-2022 Red Hat, Inc.
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

package config

import (
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// defaultConfig represents the default configuration for the DevWorkspace Operator.
var defaultConfig = &v1alpha1.OperatorConfiguration{
	Routing: &v1alpha1.RoutingConfig{
		DefaultRoutingClass: "basic",
		ClusterHostSuffix:   "", // is auto discovered when running on OpenShift. Must be defined by CR on Kubernetes.
	},
	Workspace: &v1alpha1.WorkspaceConfig{
		ImagePullPolicy: "Always",
		PVCName:         "claim-devworkspace",
		DefaultStorageSize: &v1alpha1.StorageSizes{
			Common:       &commonStorageSize,
			PerWorkspace: &perWorkspaceStorageSize,
		},
		IdleTimeout:     "15m",
		ProgressTimeout: "5m",
		CleanupOnStop:   &boolFalse,
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser:    &int64UID,
			RunAsGroup:   &int64GID,
			RunAsNonRoot: &boolTrue,
			FSGroup:      &int64UID,
		},
	},
}

// Necessary variables for setting pointer values
var (
	boolTrue                = true
	boolFalse               = false
	int64UID                = int64(1234)
	int64GID                = int64(0)
	commonStorageSize       = resource.MustParse("10Gi")
	perWorkspaceStorageSize = resource.MustParse("5Gi")
)
