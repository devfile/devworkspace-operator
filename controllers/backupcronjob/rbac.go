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
	"github.com/devfile/devworkspace-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	JobRunnerSAName = "devworkspace-job-runner"
)

func (r *BackupCronJobReconciler) ensureJobRunnerRBAC(ctx context.Context, workspace *dw.DevWorkspace) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: JobRunnerSAName + "-" + workspace.Status.DevWorkspaceId, Namespace: workspace.Namespace, Labels: map[string]string{
			constants.DevWorkspaceIDLabel:          workspace.Status.DevWorkspaceId,
			constants.DevWorkspaceWatchSecretLabel: "true",
		}},
	}

	// Create or update ServiceAccount
	if err := controllerutil.SetControllerReference(workspace, sa, r.Scheme); err != nil {
		return err
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error { return nil }); err != nil {
		return fmt.Errorf("ensuring ServiceAccount: %w", err)
	}

	return nil

}
