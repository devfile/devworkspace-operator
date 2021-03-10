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

package metadata

import (
	"fmt"
	"time"
)

// NotReadyError represents the state where no unexpected issues occurred but the provisioning
// required for the DevWorkspace is not ready
type NotReadyError struct {
	// Message is a user-friendly string explaining why the error occurred
	Message string
	// RequeueAfter represents how long we should wait before checking if storage is ready
	RequeueAfter time.Duration
}

func (e *NotReadyError) Error() string {
	return e.Message
}

// ProvisioningError represents an unrecoverable issue in provisioning a DevWorkspace.
type ProvisioningError struct {
	// Err is the underlying error causing the problem. If nil, it is not included in the output of Error()
	Err error
	// Message is a user-friendly string explaining why the error occurred
	Message string
}

func (e *ProvisioningError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err)
	}
	return e.Message
}
