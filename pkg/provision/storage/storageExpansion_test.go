// Copyright (c) 2019-2022 Red Hat, Inc.
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

package storage

import (
	"context"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(storagev1.AddToScheme(scheme))
}

func runExpansionTestCases(test testCase, t *testing.T, storageProvisioner Provisioner, PVC *corev1.PersistentVolumeClaim, volumeExpansionAllowed bool, expectedPVCSize resource.Quantity) {
	workspace := dw.DevWorkspace{}
	workspace.Spec.Template = *test.Input.Workspace
	workspace.Status.DevWorkspaceId = test.Input.DevWorkspaceID
	workspace.Namespace = "test-namespace"

	workspaceWithConfig := &common.DevWorkspaceWithConfig{}
	workspaceWithConfig.DevWorkspace = &workspace
	workspaceWithConfig.Config = &test.Input.ControllerConfig

	storageClass := &storagev1.StorageClass{}
	storageClass.Name = *workspaceWithConfig.Config.Workspace.StorageClassName
	storageClass.Namespace = PVC.Namespace
	storageClass.AllowVolumeExpansion = &volumeExpansionAllowed

	clusterAPI := sync.ClusterAPI{
		Scheme: scheme,
		// The PVC already exists in the cluster at the original size
		Client: fake.NewFakeClientWithScheme(scheme, PVC),
		Logger: zap.New(),
	}

	err := clusterAPI.Client.Create(context.TODO(), storageClass)
	assert.NoError(t, err, "Unable to add storage class to testing cluster")

	err = storageProvisioner.ProvisionStorage(&test.Input.PodAdditions, workspaceWithConfig, clusterAPI)

	if !assert.NoError(t, err, "Should not return error") {
		return
	}

	retrievedPVC := &corev1.PersistentVolumeClaim{}
	namespacedName := types.NamespacedName{Name: PVC.Name, Namespace: workspace.Namespace}

	err = clusterAPI.Client.Get(clusterAPI.Ctx, namespacedName, retrievedPVC)
	assert.NoError(t, err, "Unable to retrieve modified PVC")

	// TODO: Use cmp instead of assert.Equal?

	// Case 1: Experimental Features enabled, volume expansion allowed, and final PVC size is not the desired PVC size
	if *test.Input.ControllerConfig.EnableExperimentalFeatures && volumeExpansionAllowed && assert.Equal(t, retrievedPVC.Spec.Resources.Requests[corev1.ResourceStorage], expectedPVCSize, "PVC size should have increased") {
		return
	}

	// Case 2: Experimental Features enabled, volume expansion not allowed, yet the PVC size still changed
	// Note: On a real cluster, this is impossible to happen. This case only ensures there is a check for whether volume expansion is allowedvolume expansion not allowed
	if *test.Input.ControllerConfig.EnableExperimentalFeatures && !volumeExpansionAllowed && assert.NotEqual(t, retrievedPVC.Spec.Resources.Requests[corev1.ResourceStorage], expectedPVCSize, "Volume expansion not allowed; PVC size should not have changed") {
		return
	}

	// Case 3: Experimental Features disabled, yet the PVC size still changed
	if !*test.Input.ControllerConfig.EnableExperimentalFeatures && assert.NotEqual(t, retrievedPVC.Spec.Resources.Requests[corev1.ResourceStorage], expectedPVCSize, "ExperimentalFeatures disabled; PVC size should not have changed") {
		return
	}
}

func TestExpandCommonStorage(t *testing.T) {
	test := loadTestCaseOrPanic(t, "testdata/storage-expansion/do-expand-storage.yaml")

	t.Run(test.Name, func(t *testing.T) {
		// sanity check that file is read correctly.
		assert.NotNil(t, test.Input.Workspace, "Input does not define workspace")
		assert.NotNil(t, test.Input.ControllerConfig, "Input does not define an operator configuration")

		storageProvisioner := CommonStorageProvisioner{}
		PVC := getPVCSpec("claim-devworkspace", "test-namespace", test.Input.ControllerConfig.Workspace.StorageClassName, resource.MustParse("10Gi"))
		PVC.Status.Phase = corev1.ClaimBound

		runExpansionTestCases(test, t, &storageProvisioner, PVC, true, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.Common)
		runExpansionTestCases(test, t, &storageProvisioner, PVC, false, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.Common)
	})
}

func TestExpandCommonStorageExperimentalFeaturesDisabled(t *testing.T) {
	test := loadTestCaseOrPanic(t, "testdata/storage-expansion/do-not-expand-storage.yaml")

	t.Run(test.Name, func(t *testing.T) {
		// sanity check that file is read correctly.
		assert.NotNil(t, test.Input.Workspace, "Input does not define workspace")
		assert.NotNil(t, test.Input.ControllerConfig, "Input does not define an operator configuration")

		storageProvisioner := CommonStorageProvisioner{}
		PVC := getPVCSpec("claim-devworkspace", "test-namespace", test.Input.ControllerConfig.Workspace.StorageClassName, resource.MustParse("10Gi"))
		PVC.Status.Phase = corev1.ClaimBound

		runExpansionTestCases(test, t, &storageProvisioner, PVC, true, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.Common)
		runExpansionTestCases(test, t, &storageProvisioner, PVC, false, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.Common)
	})
}

func TestExpandPerWorkspaceStorage(t *testing.T) {
	test := loadTestCaseOrPanic(t, "testdata/storage-expansion/do-expand-storage.yaml")

	t.Run(test.Name, func(t *testing.T) {
		// sanity check that file is read correctly.
		assert.NotNil(t, test.Input.Workspace, "Input does not define workspace")
		assert.NotNil(t, test.Input.ControllerConfig, "Input does not define an operator configuration")

		storageProvisioner := PerWorkspaceStorageProvisioner{}
		PVC := getPVCSpec(common.PerWorkspacePVCName(test.Input.DevWorkspaceID), "test-namespace", test.Input.ControllerConfig.Workspace.StorageClassName, resource.MustParse("10Gi"))
		PVC.Status.Phase = corev1.ClaimBound

		runExpansionTestCases(test, t, &storageProvisioner, PVC, true, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.PerWorkspace)
		runExpansionTestCases(test, t, &storageProvisioner, PVC, false, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.PerWorkspace)
	})
}

func TestExpandPerWorkspaceStorageExperimentalFeaturesDisabled(t *testing.T) {
	test := loadTestCaseOrPanic(t, "testdata/storage-expansion/do-not-expand-storage.yaml")

	t.Run(test.Name, func(t *testing.T) {
		// sanity check that file is read correctly.
		assert.NotNil(t, test.Input.Workspace, "Input does not define workspace")
		assert.NotNil(t, test.Input.ControllerConfig, "Input does not define an operator configuration")

		storageProvisioner := PerWorkspaceStorageProvisioner{}
		PVC := getPVCSpec(common.PerWorkspacePVCName(test.Input.DevWorkspaceID), "test-namespace", test.Input.ControllerConfig.Workspace.StorageClassName, resource.MustParse("10Gi"))
		PVC.Status.Phase = corev1.ClaimBound

		runExpansionTestCases(test, t, &storageProvisioner, PVC, true, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.PerWorkspace)
		runExpansionTestCases(test, t, &storageProvisioner, PVC, false, *test.Input.ControllerConfig.Workspace.DefaultStorageSize.PerWorkspace)
	})
}
