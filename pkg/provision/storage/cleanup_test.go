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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
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

func TestGetSpecCommonPVCCleanupJobHasNodeAffinityWhenPodMountsPVC(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	namespace := "test-ns"
	pvcName := "claim-devworkspace"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-pod",
			Namespace: namespace,
			Labels:    map[string]string{constants.DevWorkspaceIDLabel: "other-workspace-id"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "other-workspace",
				UID:        "rs-uid",
				Controller: func() *bool { b := true; return &b }(),
			}},
		},
		Spec:   corev1.PodSpec{NodeName: "worker-node-1", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		pod,
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
				PVCName: pvcName,
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
	assert.NotNil(t, job.Spec.Template.Spec.Affinity)
	assert.NotNil(t, job.Spec.Template.Spec.Affinity.NodeAffinity)
	nodeSelector := job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	assert.NotNil(t, nodeSelector)
	assert.Len(t, nodeSelector.NodeSelectorTerms, 1)
	assert.Len(t, nodeSelector.NodeSelectorTerms[0].MatchExpressions, 1)
	expr := nodeSelector.NodeSelectorTerms[0].MatchExpressions[0]
	assert.Equal(t, "kubernetes.io/hostname", expr.Key)
	assert.Equal(t, corev1.NodeSelectorOpIn, expr.Operator)
	assert.Equal(t, []string{"worker-node-1"}, expr.Values)
}

func TestGetSpecCommonPVCCleanupJobWithNilPodSecurityContext(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

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
				PodSecurityContext: nil, // No custom security context
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
	assert.Nil(t, job.Spec.Template.Spec.SecurityContext)
}

// makeTerminatingWorkspace returns a DevWorkspace that is being deleted, for use as the
// workspace whose storage cleanup is running.
func makeTerminatingWorkspace(name, namespace string) *dw.DevWorkspace {
	now := metav1.Now()
	return &dw.DevWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(name + "-uid"),
			DeletionTimestamp: &now,
			Finalizers:        []string{constants.StorageCleanupFinalizer},
		},
		Status: dw.DevWorkspaceStatus{
			DevWorkspaceId: name + "-id",
		},
	}
}

// makeCleanupJob returns a cleanup Job fixture labelled with the workspace ID label and
// mounting the given PVC. If ownerUID is set, the job is owned by the DevWorkspace with
// that UID; if complete is set, the job has a JobComplete condition.
func makeCleanupJob(name, namespace, workspaceId, pvcName, ownerUID string, complete bool) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{constants.DevWorkspaceIDLabel: workspaceId},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: pvcName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
						},
					}},
				},
			},
		},
	}
	if ownerUID != "" {
		job.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "workspace.devfile.io/v1alpha2",
			Kind:       "DevWorkspace",
			Name:       "owner-workspace",
			UID:        types.UID(ownerUID),
			Controller: func() *bool { b := true; return &b }(),
		}}
	}
	if complete {
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}}
	}
	return job
}

// makeCleanupPod returns a pod fixture labelled with the workspace ID label and mounting
// the given PVC. If ownerJobName is set, the pod is controlled by the Job with that name.
func makeCleanupPod(name, namespace, workspaceId, pvcName, ownerJobName string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{constants.DevWorkspaceIDLabel: workspaceId},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: pvcName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
	}
	if ownerJobName != "" {
		pod.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       ownerJobName,
			UID:        types.UID(ownerJobName + "-uid"),
			Controller: func() *bool { b := true; return &b }(),
		}}
	}
	return pod
}

func TestCleanupWorkspaceStorageCoordinatesSharedPVCDeletion(t *testing.T) {
	const namespace = "test-ns"
	const pvcName = "claim-devworkspace"

	makeClusterAPI := func(objects ...client.Object) (sync.ClusterAPI, client.Client) {
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objects...).
			Build()
		return sync.ClusterAPI{
			Client: fakeClient,
			Scheme: scheme,
			Logger: zap.New(zap.UseDevMode(true)),
			Ctx:    context.Background(),
		}, fakeClient
	}

	makeWorkspaceWithConfig := func(workspace *dw.DevWorkspace) *common.DevWorkspaceWithConfig {
		return &common.DevWorkspaceWithConfig{
			DevWorkspace: workspace,
			Config: &v1alpha1.OperatorConfiguration{
				Workspace: &v1alpha1.WorkspaceConfig{PVCName: pvcName},
			},
		}
	}

	getPVC := func(cli client.Client) error {
		return cli.Get(context.Background(), types.NamespacedName{Name: pvcName, Namespace: namespace}, &corev1.PersistentVolumeClaim{})
	}

	makeSharedPVC := func() *corev1.PersistentVolumeClaim {
		pvc, err := getPVCSpec(pvcName, namespace, nil, resource.MustParse("10Gi"), nil)
		if err != nil {
			t.Fatal(err)
		}
		return pvc
	}

	t.Run("deletes PVC when no jobs or pods reference it", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)
		assert.NoError(t, err)
		assert.True(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should be deleted")
	})

	t.Run("waits for in-flight cleanup job owned by another workspace", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		otherWorkspace := makeTerminatingWorkspace("ws-other", namespace)
		inFlightJob := makeCleanupJob("cleanup-other", namespace, "ws-other-id", pvcName, "ws-other-uid", false)
		jobPod := makeCleanupPod("cleanup-other-pod", namespace, "ws-other-id", pvcName, "cleanup-other")
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, otherWorkspace, inFlightJob, jobPod)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		var retryErr *dwerrors.RetryError
		assert.ErrorAs(t, err, &retryErr, "Should return RetryError while cleanup job is in flight")
		assert.False(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should still exist")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "cleanup-other", Namespace: namespace}, &batchv1.Job{}),
			"In-flight job should not be deleted")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "cleanup-other-pod", Namespace: namespace}, &corev1.Pod{}),
			"Pod of in-flight job should not be deleted")
	})

	t.Run("waits for running cleanup job whose owning workspace is already deleted", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		// The job's owning workspace is gone from the cluster, but the job is still
		// running and its pod still references the shared PVC.
		runningJob := makeCleanupJob("cleanup-orphaned-owner", namespace, "ws-gone-id", pvcName, "ws-gone-uid", false)
		jobPod := makeCleanupPod("cleanup-orphaned-owner-pod", namespace, "ws-gone-id", pvcName, "cleanup-orphaned-owner")
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, runningJob, jobPod)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		var retryErr *dwerrors.RetryError
		assert.ErrorAs(t, err, &retryErr, "Should return RetryError while job of deleted workspace is still running")
		assert.Contains(t, retryErr.Message, "cleanup-orphaned-owner", "Wait message should mention the owning job for debuggability")
		assert.False(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should still exist")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "cleanup-orphaned-owner", Namespace: namespace}, &batchv1.Job{}),
			"Running job should not be deleted")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "cleanup-orphaned-owner-pod", Namespace: namespace}, &corev1.Pod{}),
			"Pod of running job should not be deleted")
	})

	t.Run("deletes orphaned cleanup pods before deleting PVC", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		orphanedPod := makeCleanupPod("orphaned-cleanup-pod", namespace, "ws-gone-id", pvcName, "")
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, orphanedPod)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		// The orphaned pod is deleted, but the PVC deletion is requeued to confirm on
		// the next reconcile that no references remain.
		var retryErr *dwerrors.RetryError
		assert.ErrorAs(t, err, &retryErr, "Should return RetryError after deleting orphaned pod")
		assert.True(t, k8sErrors.IsNotFound(fakeClient.Get(context.Background(),
			types.NamespacedName{Name: "orphaned-cleanup-pod", Namespace: namespace}, &corev1.Pod{})), "Orphaned pod should be deleted")
		assert.False(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should still exist")

		err = (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)
		assert.NoError(t, err)
		assert.True(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should be deleted once no pods reference it")
	})

	t.Run("deletes pods of finished cleanup jobs before deleting PVC", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		finishedJob := makeCleanupJob("cleanup-done", namespace, "ws-gone-id", pvcName, "", true)
		jobPod := makeCleanupPod("cleanup-done-pod", namespace, "ws-gone-id", pvcName, "cleanup-done")
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, finishedJob, jobPod)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		// The finished job's pod is deleted, but the PVC deletion is requeued to confirm on
		// the next reconcile that no references remain.
		var retryErr *dwerrors.RetryError
		assert.ErrorAs(t, err, &retryErr, "Should return RetryError after deleting finished job's pod")
		assert.True(t, k8sErrors.IsNotFound(fakeClient.Get(context.Background(),
			types.NamespacedName{Name: "cleanup-done-pod", Namespace: namespace}, &corev1.Pod{})), "Finished job's pod should be deleted")
		assert.False(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should still exist")

		err = (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)
		assert.NoError(t, err)
		assert.True(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should be deleted once no pods reference it")
	})

	t.Run("ignores jobs and pods that do not mount the shared PVC", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		otherJob := makeCleanupJob("async-job", namespace, "ws-other-id", "claim-async", "ws-other-uid", false)
		otherPod := makeCleanupPod("async-pod", namespace, "ws-other-id", "claim-async", "")
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, otherJob, otherPod)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		assert.NoError(t, err)
		assert.True(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should be deleted")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "async-job", Namespace: namespace}, &batchv1.Job{}),
			"Job mounting another PVC should not be deleted")
		assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "async-pod", Namespace: namespace}, &corev1.Pod{}),
			"Pod mounting another PVC should not be deleted")
	})

	t.Run("does not wait for finished cleanup job before deleting PVC", func(t *testing.T) {
		currentWorkspace := makeTerminatingWorkspace("ws-current", namespace)
		completedJob := makeCleanupJob("cleanup-done", namespace, "ws-other-id", pvcName, "ws-other-uid", true)
		clusterAPI, fakeClient := makeClusterAPI(makeSharedPVC(), currentWorkspace, completedJob)

		err := (&CommonStorageProvisioner{}).CleanupWorkspaceStorage(makeWorkspaceWithConfig(currentWorkspace), clusterAPI)

		assert.NoError(t, err)
		assert.True(t, k8sErrors.IsNotFound(getPVC(fakeClient)), "PVC should be deleted despite finished job still referencing it in spec")
	})
}

func TestIsJobFinished(t *testing.T) {
	jobWithCondition := func(conditionType batchv1.JobConditionType, status corev1.ConditionStatus) *batchv1.Job {
		return &batchv1.Job{
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: conditionType, Status: status}},
			},
		}
	}

	assert.True(t, isJobFinished(jobWithCondition(batchv1.JobComplete, corev1.ConditionTrue)), "Completed job should be finished")
	assert.True(t, isJobFinished(jobWithCondition(batchv1.JobFailed, corev1.ConditionTrue)), "Failed job should be finished")
	assert.False(t, isJobFinished(jobWithCondition(batchv1.JobComplete, corev1.ConditionFalse)), "Job with non-true Complete condition should not be finished")
	assert.False(t, isJobFinished(jobWithCondition(batchv1.JobSuspended, corev1.ConditionTrue)), "Suspended job should not be finished")
	assert.False(t, isJobFinished(&batchv1.Job{}), "Job without conditions should not be finished")
}
