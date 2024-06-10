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
	Webhook: &v1alpha1.WebhookConfig{
		Replicas: pointer.Int32(2),
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
		PersistUserHome: &v1alpha1.PersistentHomeConfig{
			Enabled:              pointer.Bool(false),
			DisableInitContainer: pointer.Bool(false),
		},
		IdleTimeout:              "15m",
		ProgressTimeout:          "5m",
		CleanupOnStop:            pointer.Bool(false),
		PodSecurityContext:       nil, // Set per-platform in setDefaultPodSecurityContext()
		ContainerSecurityContext: nil, // Set per-platform in setDefaultContainerSecurityContext()
		DefaultTemplate:          nil,
		ProjectCloneConfig: &v1alpha1.ProjectCloneConfig{
			Resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
					corev1.ResourceCPU:    resource.MustParse("1000m"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
					corev1.ResourceCPU:    resource.MustParse("100m"),
				},
			},
		},
		DefaultContainerResources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	},
}

var (
	defaultKubernetesPodSecurityContext = &corev1.PodSecurityContext{
		RunAsUser:    pointer.Int64(1234),
		RunAsGroup:   pointer.Int64(0),
		RunAsNonRoot: pointer.Bool(true),
		FSGroup:      pointer.Int64(1234),
	}
	defaultKubernetesContainerSecurityContext = &corev1.SecurityContext{}
	defaultOpenShiftPodSecurityContext        = &corev1.PodSecurityContext{}
	defaultOpenShiftContainerSecurityContext  = &corev1.SecurityContext{
		ReadOnlyRootFilesystem:   pointer.Bool(false),
		RunAsNonRoot:             pointer.Bool(true),
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
	}
)

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

func setDefaultContainerSecurityContext() error {
	if !infrastructure.IsInitialized() {
		return fmt.Errorf("can not set default container security context, infrastructure not detected")
	}
	if infrastructure.IsOpenShift() {
		defaultConfig.Workspace.ContainerSecurityContext = defaultOpenShiftContainerSecurityContext
	} else {
		defaultConfig.Workspace.ContainerSecurityContext = defaultKubernetesContainerSecurityContext
	}
	return nil
}
