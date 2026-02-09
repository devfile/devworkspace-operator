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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
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
	secretName := dwOperatorConfig.Workspace.BackupCronJob.Registry.AuthSecret
	if secretName == "" {
		// No auth secret configured - anonymous access to registry
		return nil, nil
	}

	// First check the workspace namespace for the secret
	registryAuthSecret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: workspace.Namespace}, registryAuthSecret)
	if err == nil {
		log.Info("Successfully retrieved registry auth secret for backup from workspace namespace", "secretName", secretName)
		return registryAuthSecret, nil
	}
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	// If we don't provide an operator namespace, don't attempt to look there
	if operatorConfigNamespace == "" {
		return nil, nil
	}
	log.Info("Registry auth secret not found in workspace namespace, checking operator namespace", "secretName", secretName)

	// If the secret is not found in the workspace namespace, check the operator namespace as fallback
	err = c.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: operatorConfigNamespace}, registryAuthSecret)
	if err != nil {
		log.Error(err, "Failed to get registry auth secret for backup job", "secretName", secretName)
		return nil, err
	}
	log.Info("Successfully retrieved registry auth secret for backup job", "secretName", secretName)
	return CopySecret(ctx, c, workspace, registryAuthSecret, scheme, log)
}

// CopySecret copies the given secret from the operator namespace to the workspace namespace.
func CopySecret(ctx context.Context, c client.Client, workspace *dw.DevWorkspace, sourceSecret *corev1.Secret, scheme *runtime.Scheme, log logr.Logger) (namespaceSecret *corev1.Secret, err error) {
	// Construct the desired secret state
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

	if err := controllerutil.SetControllerReference(workspace, desiredSecret, scheme); err != nil {
		return nil, err
	}

	// Use the sync mechanism
	clusterAPI := sync.ClusterAPI{
		Client: c,
		Scheme: scheme,
		Logger: log,
		Ctx:    ctx,
	}

	syncedObj, err := sync.SyncObjectWithCluster(desiredSecret, clusterAPI)
	if err != nil {
		if _, ok := err.(*sync.NotInSyncError); !ok {
			return nil, err
		}
		// NotInSyncError means the sync operation was successful but triggered a change
		log.Info("Successfully synced secret", "name", desiredSecret.Name, "namespace", workspace.Namespace)
	}

	// If syncedObj is nil (due to NotInSyncError), return the desired object
	if syncedObj == nil {
		return desiredSecret, nil
	}

	return syncedObj.(*corev1.Secret), nil
}
