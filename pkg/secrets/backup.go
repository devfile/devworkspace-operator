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

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func HandleRegistryAuthSecret(ctx context.Context, c client.Client, workspace *dw.DevWorkspace,
	dwOperatorConfig *controllerv1alpha1.OperatorConfiguration, operatorConfigNamespace string, scheme *runtime.Scheme, log logr.Logger,
) (*corev1.Secret, error) {
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
	existingNamespaceSecret := &corev1.Secret{}
	err = c.Get(ctx, client.ObjectKey{
		Name:      constants.DevWorkspaceBackupAuthSecretName,
		Namespace: workspace.Namespace}, existingNamespaceSecret)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to check for existing registry auth secret in workspace namespace", "namespace", workspace.Namespace)
		return nil, err
	}
	if err == nil {
		err = c.Delete(ctx, existingNamespaceSecret)
		if err != nil {
			return nil, err
		}
	}
	namespaceSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:          workspace.Status.DevWorkspaceId,
				constants.DevWorkspaceWatchSecretLabel: "true",
			},
		},
		Data: sourceSecret.Data,
		Type: sourceSecret.Type,
	}
	if err := controllerutil.SetControllerReference(workspace, namespaceSecret, scheme); err != nil {
		return nil, err
	}
	err = c.Create(ctx, namespaceSecret)
	if err == nil {
		log.Info("Successfully created secret", "name", namespaceSecret.Name, "namespace", workspace.Namespace)
	}
	return namespaceSecret, err
}
