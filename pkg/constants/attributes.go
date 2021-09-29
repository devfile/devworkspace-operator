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
package constants

// Constants that are used in attributes on DevWorkspace elements (components, endpoints, etc.)
const (
	// PluginSourceAttribute is an attribute added to components, commands, and projects in a flattened
	// DevWorkspace representation to signify where the respective component came from (i.e. which plugin
	// or parent imported it)
	PluginSourceAttribute = "controller.devfile.io/imported-by"
	// EndpointURLAttribute is an attribute added to endpoints to denote the endpoint on the cluster that
	// was created to route to this endpoint
	EndpointURLAttribute = "controller.devfile.io/endpoint-url"
)
