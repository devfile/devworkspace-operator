//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

import "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"

// DefaultConfig represents the default configuration for the DevWorkspace Operator.
var DefaultConfig = &v1alpha1.OperatorConfiguration{
	Routing: &v1alpha1.RoutingConfig{
		DefaultRoutingClass: "basic",
		ClusterHostSuffix:   "", // is auto discovered when running on OpenShift. Must be defined by CR on Kubernetes.
	},
	Workspace: &v1alpha1.WorkspaceConfig{
		ImagePullPolicy: "Always",
		PVCName:         "claim-devworkspace",
		IdleTimeout:     "15m",
	},
}
