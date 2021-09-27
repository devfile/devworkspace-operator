//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
		ProgressTimeout: "5m",
	},
}
