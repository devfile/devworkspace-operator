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

package solvers

import (
	"errors"
	"time"
)

var _ error = (*RoutingNotReady)(nil)
var _ error = (*RoutingInvalid)(nil)

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
