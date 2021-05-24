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
