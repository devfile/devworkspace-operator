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

package provision

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProvisioningStatus struct {
	// Continue should be true if cluster state matches spec state for this step
	Continue    bool
	Requeue     bool
	FailStartup bool
	Err         error
}

type ClusterAPI struct {
	Client client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
}
