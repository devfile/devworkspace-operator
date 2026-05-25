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

package storage

import (
	"context"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

func TestGetSpecCommonPVCCleanupJobUsesConfigPodSecurityContext(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	fsGroupChangeOnRootMismatch := corev1.FSGroupChangeOnRootMismatch
	customPodSecurityContext := &corev1.PodSecurityContext{
		FSGroupChangePolicy: &fsGroupChangeOnRootMismatch,
		SELinuxOptions:      &corev1.SELinuxOptions{Type: "spc_t"},
	}

	namespace := "test-ns"
	pvcName := "claim-devworkspace"
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
	).Build()

	workspace := &common.DevWorkspaceWithConfig{
		DevWorkspace: &dw.DevWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workspace",
				Namespace: namespace,
				Labels: map[string]string{
					constants.DevWorkspaceCreatorLabel: "test-creator",
				},
			},
			Status: dw.DevWorkspaceStatus{
				DevWorkspaceId: "test-workspace-id",
			},
		},
		Config: &v1alpha1.OperatorConfiguration{
			Workspace: &v1alpha1.WorkspaceConfig{
				PVCName:            pvcName,
				PodSecurityContext: customPodSecurityContext,
			},
		},
	}

	clusterAPI := sync.ClusterAPI{
		Client: fakeClient,
		Scheme: scheme,
		Logger: zap.New(zap.UseDevMode(true)),
		Ctx:    context.Background(),
	}

	job, err := getSpecCommonPVCCleanupJob(workspace, clusterAPI)
	assert.NoError(t, err)
	assert.Equal(t, customPodSecurityContext, job.Spec.Template.Spec.SecurityContext)
}
