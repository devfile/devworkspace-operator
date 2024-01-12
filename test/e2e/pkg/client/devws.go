//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"errors"
	"log"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
)

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
			log.Printf("Now current status of developer workspace is: %s. Message: %s", currentStatus.Phase, currentStatus.Message)
			if currentStatus.Phase == dw.DevWorkspaceStatusFailed {
				return false, errors.New("workspace has been failed unexpectedly. Message: " + currentStatus.Message)
			}
			if currentStatus.Phase == expectedStatus {
				return true, nil
			}
		}
	}
}
