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
