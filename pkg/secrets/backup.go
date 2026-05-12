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

package secrets

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetRegistryAuthSecret retrieves the registry authentication secret for accessing backup images
// based on the operator configuration.
func GetNamespaceRegistryAuthSecret(ctx context.Context, c client.Client, workspace *dw.DevWorkspace,
	dwOperatorConfig *controllerv1alpha1.OperatorConfiguration, scheme *runtime.Scheme, log logr.Logger,
) (*corev1.Secret, error) {
	return HandleRegistryAuthSecret(ctx, c, workspace, dwOperatorConfig, "", scheme, log)
}

func HandleRegistryAuthSecret(ctx context.Context, c client.Client, workspace *dw.DevWorkspace,
	dwOperatorConfig *controllerv1alpha1.OperatorConfiguration, operatorConfigNamespace string, scheme *runtime.Scheme, log logr.Logger,
) (*corev1.Secret, error) {
	if dwOperatorConfig.Workspace == nil ||
		dwOperatorConfig.Workspace.BackupCronJob == nil ||
		dwOperatorConfig.Workspace.BackupCronJob.Registry == nil {
		return nil, fmt.Errorf("backup/restore configuration not properly set in DevWorkspaceOperatorConfig")
	}

	registryAuthSecret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      constants.DevWorkspaceBackupAuthSecretName,
		Namespace: workspace.Namespace}, registryAuthSecret)
	if err == nil {
		log.Info("Successfully retrieved registry auth secret for backup from workspace namespace",
			"secretName", constants.DevWorkspaceBackupAuthSecretName)
		return registryAuthSecret, nil
	}
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	if operatorConfigNamespace == "" {
		resolvedNS, nsErr := infrastructure.GetNamespace()
		if nsErr != nil {
			log.Info("Cannot resolve operator namespace for auth secret fallback", "error", nsErr)
			return nil, nil
		}
		operatorConfigNamespace = resolvedNS
	}

	// Check if AuthSecret is configured in operator config
	authSecretName := dwOperatorConfig.Workspace.BackupCronJob.Registry.AuthSecret
	if len(authSecretName) == 0 {
		log.Info("Registry auth secret not found in workspace namespace and not configured in operator config. "+
			"Proceeding without authentication. Ensure your registry allows anonymous access or configure authentication if needed.",
			"secretName", constants.DevWorkspaceBackupAuthSecretName,
			"namespace", workspace.Namespace,
			"registry", dwOperatorConfig.Workspace.BackupCronJob.Registry.Path)
		return nil, nil
	}

	log.Info("Registry auth secret not found in workspace namespace, checking operator namespace",
		"secretName", authSecretName,
		"operatorNamespace", operatorConfigNamespace)

	// Look for the configured secret name in operator namespace
	err = c.Get(ctx, client.ObjectKey{
		Name:      authSecretName,
		Namespace: operatorConfigNamespace}, registryAuthSecret)
	if err != nil {
		log.Error(err, "Failed to get registry auth secret for backup job",
			"secretName", authSecretName,
			"namespace", operatorConfigNamespace)
		return nil, err
	}
	log.Info("Successfully retrieved registry auth secret from operator namespace",
		"secretName", authSecretName)
	return CopySecret(ctx, c, workspace, registryAuthSecret, scheme, log)
}

// CopySecret copies the given secret from the operator namespace to the workspace namespace.
// It NEVER overwrites an existing secret: if a secret already exists in the workspace namespace,
// it returns the existing secret without modification.
func CopySecret(ctx context.Context, c client.Client, workspace *dw.DevWorkspace, sourceSecret *corev1.Secret, scheme *runtime.Scheme, log logr.Logger) (namespaceSecret *corev1.Secret, err error) {
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
		},
		Data: sourceSecret.Data,
		Type: sourceSecret.Type,
	}

	err = c.Create(ctx, desiredSecret)
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			// Race condition - secret was created between Get and Create
			// Fetch and return it (respect what's there)
			if err := c.Get(ctx, client.ObjectKey{
				Name:      constants.DevWorkspaceBackupAuthSecretName,
				Namespace: workspace.Namespace,
			}, sourceSecret); err != nil {
				return nil, err
			}
			log.Info("Registry auth secret was created concurrently, using existing secret",
				"secretName", constants.DevWorkspaceBackupAuthSecretName)
			return sourceSecret, nil
		}
		return nil, err
	}

	log.Info("Successfully copied registry auth secret to workspace namespace",
		"name", desiredSecret.Name, "namespace", workspace.Namespace)
	return desiredSecret, nil
}
