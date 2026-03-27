//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package solvers

import (
	"errors"
	"fmt"
	"time"
)

var _ error = (*RoutingNotReady)(nil)
var _ error = (*RoutingInvalid)(nil)
var _ error = (*ServiceConflictError)(nil)

// ServiceConflictError is returned when a discoverable endpoint has a name that is already in use by
// another DevWorkspace's service.
type ServiceConflictError struct {
	EndpointName  string
	WorkspaceName string
}

func (e *ServiceConflictError) Error() string {
	if e.WorkspaceName == "" {
		return fmt.Sprintf("discoverable endpoint '%s' is already in use by another workspace", e.EndpointName)
	}
	return fmt.Sprintf("discoverable endpoint '%s' is already in use by workspace '%s'", e.EndpointName, e.WorkspaceName)
}

// RoutingNotSupported is used by the solvers when they supported the routingclass of the workspace they've been asked to route
var RoutingNotSupported = errors.New("routingclass not supported by this controller")

// RoutingNotReady is used by the solvers when they are not ready to route an otherwise OK workspace. They can also suggest the
// duration after which to retry the workspace routing. If not specified, the retry is made after 1 second.
type RoutingNotReady struct {
	Retry time.Duration
}

func (*RoutingNotReady) Error() string {
	return "controller not ready to resolve the workspace routing"
}

// RoutingInvalid is used by the solvers to report that they were asked to route a workspace that has the correct routingclass but
// is invalid in some other sense - missing configuration, etc.
type RoutingInvalid struct {
	Reason string
}

func (e *RoutingInvalid) Error() string {
	reason := "<no reason given>"
	if len(e.Reason) > 0 {
		reason = e.Reason
	}
	return "workspace routing is invalid: " + reason
}
