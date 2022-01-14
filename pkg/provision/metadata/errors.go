//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
