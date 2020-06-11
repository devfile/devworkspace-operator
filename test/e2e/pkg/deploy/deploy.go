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
	"github.com/devfile/devworkspace-operator/test/e2e/pkg/client"
)

type Deployment struct {
	kubeClient *client.K8sClient
}

func NewDeployment(kubeClient *client.K8sClient) *Deployment {
	return &Deployment{kubeClient: kubeClient}
}
