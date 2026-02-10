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

package controllers

import (
	"context"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	JobRunnerSAName = "devworkspace-job-runner"
)

func (r *BackupCronJobReconciler) ensureJobRunnerRBAC(ctx context.Context, workspace *dw.DevWorkspace) error {
	saName := JobRunnerSAName + "-" + workspace.Status.DevWorkspaceId
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: workspace.Namespace, Labels: map[string]string{
			constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
		}},
	}

	// Create or update ServiceAccount
	if err := controllerutil.SetControllerReference(workspace, sa, r.Scheme); err != nil {
		return err
	}

	clusterAPI := sync.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: r.Log,
		Ctx:    ctx,
	}

	_, err := sync.SyncObjectWithCluster(sa, clusterAPI)
	if err != nil {
		if _, ok := err.(*sync.NotInSyncError); !ok {
			return fmt.Errorf("synchronizing ServiceAccount: %w", err)
		}
	}

	if infrastructure.IsOpenShift() {
		// Create ClusterRoleBinding for image push role
		if err := r.ensureImagePushRoleBinding(saName, workspace, clusterAPI); err != nil {
			return fmt.Errorf("ensuring image push ClusterRoleBinding: %w", err)
		}
		// Create ImageStream for backup images
		if err := r.ensureImageStreamForBackup(ctx, workspace, clusterAPI); err != nil {
			return fmt.Errorf("ensuring ImageStream for backup: %w", err)
		}
	}

	return nil

}

// ensureImagePushRoleBinding creates a ClusterRoleBinding to allow the given ServiceAccount to push images
// to the OpenShift internal registry.
func (r *BackupCronJobReconciler) ensureImagePushRoleBinding(saName string, workspace *dw.DevWorkspace, clusterAPI sync.ClusterAPI) error {
	// Create RoleBinding for system:image-builder role
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-image-builder-" + workspace.Status.DevWorkspaceId,
			Namespace: workspace.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: workspace.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     constants.RbacClusterRoleKind,
			Name:     common.RegistryImageBuilderRoleName(),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err := sync.SyncObjectWithCluster(roleBinding, clusterAPI)
	if err != nil {
		if _, ok := err.(*sync.NotInSyncError); !ok {
			return fmt.Errorf("ensuring RoleBinding: %w", err)
		}
	}

	return nil
}

// ensureImageStreamForBackup creates an ImageStream for the backup images in OpenShift in case user
// selects to use the internal registry. Push to non-existing ImageStream fails, so we need to create it first.
func (r *BackupCronJobReconciler) ensureImageStreamForBackup(ctx context.Context, workspace *dw.DevWorkspace, clusterAPI sync.ClusterAPI) error {
	// Create ImageStream for backup images using unstructured to avoid scheme conflicts
	imageStream := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "image.openshift.io/v1",
			"kind":       "ImageStream",
			"metadata": map[string]interface{}{
				"name":      workspace.Name,
				"namespace": workspace.Namespace,
				"labels": map[string]interface{}{
					constants.DevWorkspaceIDLabel: workspace.Status.DevWorkspaceId,
				},
			},
			"spec": map[string]interface{}{
				"lookupPolicy": map[string]interface{}{
					"local": true,
				},
			},
		},
	}

	imageStream.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Version: "v1",
		Kind:    "ImageStream",
	})

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, imageStream, func() error { return nil }); err != nil {
		return fmt.Errorf("ensuring ImageStream: %w", err)
	}

	return nil
}
