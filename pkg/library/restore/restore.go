//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

// Package restore defines library functions for restoring workspace data from backup images
package restore

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	devfileConstants "github.com/devfile/devworkspace-operator/pkg/library/constants"
	dwResources "github.com/devfile/devworkspace-operator/pkg/library/resources"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/internal/images"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	WorkspaceRestoreContainerName = "workspace-restore"
)

type Options struct {
	Image      string
	PullPolicy corev1.PullPolicy
	Resources  *corev1.ResourceRequirements
	Env        []corev1.EnvVar
}

func IsWorkspaceRestoreRequested(workspace *dw.DevWorkspaceTemplateSpec) bool {
	if !workspace.Attributes.Exists(constants.WorkspaceRestoreAttribute) {
		return false
	}
	enableRecovery := workspace.Attributes.GetBoolean(constants.WorkspaceRestoreAttribute, nil)
	return enableRecovery

}

// GetWorkspaceRestoreInitContainer creates an init container that restores workspace data from a backup image.
// The restore container uses the existing workspace-recovery.sh script to extract backup content.
func GetWorkspaceRestoreInitContainer(
	ctx context.Context,
	workspace *common.DevWorkspaceWithConfig,
	k8sClient client.Client,
	options Options,
	log logr.Logger,
) (*corev1.Container, error) {
	wokrspaceTempplate := &workspace.Spec.Template

	// Determine the source image for restore
	var err error
	var restoreSourceImage string
	if wokrspaceTempplate.Attributes.Exists(constants.WorkspaceRestoreSourceImageAttribute) {
		// User choose custom image specified in the attribute
		restoreSourceImage = wokrspaceTempplate.Attributes.GetString(constants.WorkspaceRestoreSourceImageAttribute, &err)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s attribute on workspace: %w", constants.WorkspaceRestoreSourceImageAttribute, err)
		}
	} else {
		if workspace.Config.Workspace.BackupCronJob == nil {
			return nil, fmt.Errorf("workspace restore requested but backup cron job configuration is missing")
		}
		if workspace.Config.Workspace.BackupCronJob.Registry == nil || workspace.Config.Workspace.BackupCronJob.Registry.Path == "" {
			return nil, fmt.Errorf("workspace restore requested but backup cron job registry is not configured")
		}
		// Use default backup image location based on workspace info
		restoreSourceImage = workspace.Config.Workspace.BackupCronJob.Registry.Path + "/" + workspace.Namespace + "/" + workspace.Name + ":latest"
	}
	if restoreSourceImage == "" {
		return nil, fmt.Errorf("empty value for attribute %s is invalid", constants.WorkspaceRestoreSourceImageAttribute)
	}

	if !hasContainerComponents(wokrspaceTempplate) {
		// Avoid adding restore init container when DevWorkspace does not define any containers
		return nil, nil
	}

	// Use the project backup image which contains the workspace-recovery.sh script
	restoreImage := images.GetProjectBackupImage()

	// Prepare environment variables for the restore script
	env := append(options.Env, []corev1.EnvVar{
		{Name: "BACKUP_IMAGE", Value: restoreSourceImage},
	}...)

	resources := dwResources.FilterResources(options.Resources)
	if err := dwResources.ValidateResources(resources); err != nil {
		return nil, fmt.Errorf("invalid resources for project clone container: %w", err)
	}

	return &corev1.Container{
		Name:      WorkspaceRestoreContainerName,
		Image:     restoreImage,
		Command:   []string{"/workspace-recovery.sh"},
		Args:      []string{"--restore"},
		Env:       env,
		Resources: *resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      devfileConstants.ProjectsVolumeName,
				MountPath: constants.DefaultProjectsSourcesRoot,
			},
		},
		ImagePullPolicy: options.PullPolicy,
	}, nil
}

func hasContainerComponents(workspace *dw.DevWorkspaceTemplateSpec) bool {
	for _, component := range workspace.Components {
		if component.Container != nil {
			return true
		}
	}
	return false
}
