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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (w *K8sClient) UpdateDevWorkspaceStarted(name, namespace string, started bool) error {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"started": started,
		},
	}
	patchData, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	target := &dw.DevWorkspace{}
	target.ObjectMeta.Name = name
	target.ObjectMeta.Namespace = namespace

	err = w.crClient.Patch(
		context.TODO(),
		target,
		client.RawPatch(types.MergePatchType, patchData),
	)
	return err
}

// get workspace current dev workspace status from the Custom Resource object
func (w *K8sClient) GetDevWsStatus(name, namespace string) (*dw.DevWorkspaceStatus, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	workspace := &dw.DevWorkspace{}
	err := w.crClient.Get(context.TODO(), namespacedName, workspace)

	if err != nil {
		return nil, err
	}
	return &workspace.Status, nil
}

func (w *K8sClient) WaitDevWsStatus(name, namespace string, expectedStatus dw.DevWorkspacePhase) (bool, error) {
	timeout := time.After(15 * time.Minute)
	tick := time.Tick(2 * time.Second)

	for {
		select {
		case <-timeout:
			return false, errors.New("timed out")
		case <-tick:
			currentStatus, err := w.GetDevWsStatus(name, namespace)
			if err != nil {
				return false, err
			}
			log.Printf("Now current status of developer workspace %s is: %s. Message: %s", name, currentStatus.Phase, currentStatus.Message)
			if currentStatus.Phase == dw.DevWorkspaceStatusFailed {
				return false, errors.New("workspace has been failed unexpectedly. Message: " + currentStatus.Message)
			}
			if currentStatus.Phase == expectedStatus {
				return true, nil
			}
		}
	}
}

// DeleteDevWorkspace deletes a DevWorkspace using the Kubernetes client.
// Returns an error if the deletion fails (ignoring NotFound errors).
func (w *K8sClient) DeleteDevWorkspace(name, namespace string) error {
	workspace := &dw.DevWorkspace{}
	workspace.ObjectMeta.Name = name
	workspace.ObjectMeta.Namespace = namespace

	err := w.crClient.Delete(context.TODO(), workspace)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
}

// WaitForPVCDeleted waits for a PVC to be fully deleted from the cluster.
// Returns true if deleted successfully, false if timeout occurred.
func (w *K8sClient) WaitForPVCDeleted(pvcName, namespace string, timeout time.Duration) (bool, error) {
	deleted := false
	err := wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
		_, err := w.Kube().CoreV1().PersistentVolumeClaims(namespace).
			Get(context.TODO(), pvcName, metav1.GetOptions{})

		if k8sErrors.IsNotFound(err) {
			deleted = true
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	return deleted, err
}

// DeleteDevWorkspaceAndWait deletes a workspace and waits for its PVC to be fully removed.
// This ensures proper cleanup and prevents PVC conflicts in subsequent tests.
func (w *K8sClient) DeleteDevWorkspaceAndWait(name, namespace string) error {
	if err := w.DeleteDevWorkspace(name, namespace); err != nil {
		return err
	}

	// Wait for shared PVC to be deleted (may take time in cloud environments)
	_, err := w.WaitForPVCDeleted("claim-devworkspace", namespace, 2*time.Minute)
	return err
}
