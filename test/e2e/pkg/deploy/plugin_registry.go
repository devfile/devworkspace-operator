//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/config"
)

func (w *Deployment) CreatePluginRegistryDeployment() (err error) {
	label := "app=che-plugin-registry"
	deployment, err := deserializePluginRegistryDeployment()

	if err != nil {
		fmt.Println("Failed to deserialize deployment")
		return err
	}

	deployment, err = w.kubeClient.Kube().AppsV1().Deployments(config.Namespace).Create(deployment)

	if err != nil {
		fmt.Println("Failed to create deployment %s: %s", deployment.Name, err)
		return err
	}

	deploy, err := w.kubeClient.PodDeployWaitUtil(label)
	if !deploy {
		fmt.Println("Che Workspaces Controller not deployed")
		return err
	}
	return nil
}

func (w *Deployment) CreatePluginRegistryService() (err error) {
	deserializeService, _ := deserializePluginRegistryService()

	_, err = w.kubeClient.Kube().CoreV1().Services(config.Namespace).Create(deserializeService)
	if err != nil {
		fmt.Println("Failed to create service %s: %s", deserializeService.Name, err)
		return err
	}
	return nil
}
