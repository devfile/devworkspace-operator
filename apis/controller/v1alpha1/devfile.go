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
package v1alpha1

type EndpointAttribute string
type EndpointType string

const (
	// TypeEndpointAttribute is an attribute used for devfile endpoints that specifies the endpoint type.
	// See EndpointType for respected values
	TypeEndpointAttribute EndpointAttribute = "type"

	// The value for `type` endpoint attribute that indicates that it should be exposed as mainUrl
	// in the workspace status
	MainEndpointType EndpointType = "main"

	// DiscoverableAttribute defines an endpoint as "discoverable", meaning that a service should be
	// created using the endpoint name (i.e. instead of generating a service name for all endpoints,
	// this endpoint should be statically accessible)
	DiscoverableAttribute EndpointAttribute = "discoverable"
)
