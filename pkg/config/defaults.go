//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// defaultConfig represents the default configuration for the DevWorkspace Operator.
var defaultConfig = &v1alpha1.OperatorConfiguration{
	Routing: &v1alpha1.RoutingConfig{
		DefaultRoutingClass: "basic",
		ClusterHostSuffix:   "", // is auto discovered when running on OpenShift. Must be defined by CR on Kubernetes.
	},
	Workspace: &v1alpha1.WorkspaceConfig{
		ImagePullPolicy:    "Always",
		DeploymentStrategy: appsv1.RecreateDeploymentStrategyType,
		PVCName:            "claim-devworkspace",
		ServiceAccount: &v1alpha1.ServiceAccountConfig{
			DisableCreation: pointer.Bool(false),
		},
		DefaultStorageSize: &v1alpha1.StorageSizes{
			Common:       &commonStorageSize,
			PerWorkspace: &perWorkspaceStorageSize,
		},
		IdleTimeout:              "15m",
		ProgressTimeout:          "5m",
		CleanupOnStop:            pointer.BoolPtr(false),
		PodSecurityContext:       nil,
		ContainerSecurityContext: &corev1.SecurityContext{},
		DefaultTemplate:          nil,
	},
}

var defaultKubernetesPodSecurityContext = &corev1.PodSecurityContext{
	RunAsUser:    pointer.Int64(1234),
	RunAsGroup:   pointer.Int64(0),
	RunAsNonRoot: pointer.Bool(true),
	FSGroup:      pointer.Int64(1234),
}

var defaultOpenShiftPodSecurityContext = &corev1.PodSecurityContext{}

// Necessary variables for setting pointer values
var (
	commonStorageSize       = resource.MustParse("10Gi")
	perWorkspaceStorageSize = resource.MustParse("5Gi")
)

func setDefaultPodSecurityContext() error {
	if !infrastructure.IsInitialized() {
		return fmt.Errorf("can not set default pod security context, infrastructure not detected")
	}
	if infrastructure.IsOpenShift() {
		defaultConfig.Workspace.PodSecurityContext = defaultOpenShiftPodSecurityContext
	} else {
		defaultConfig.Workspace.PodSecurityContext = defaultKubernetesPodSecurityContext
	}
	return nil
}
