// Copyright (c) 2019-2024 Red Hat, Inc.
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

package dwerrors

import (
	"fmt"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

type RetryError struct {
	// Err is the underlying error causing the problem. If nil, it is not included in the output of Error()
	Err error
	// Message is a user-friendly string explaining why the error occurred
	Message string
	// RequeueAfter represents how long we should wait before requeueing a reconcile. If unspecified,
	// reconcile is requeued immediately.
	RequeueAfter time.Duration
}

func (e *RetryError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s: %s", e.Message, e.Err)
		}
		return e.Err.Error()
	}
	return e.Message
}

func (e *RetryError) Unwrap() error {
	return e.Err
}

type FailError struct {
	// Err is the underlying error causing the problem. If nil, it is not included in the output of Error()
	Err error
	// Message is a user-friendly string explaining why the error occurred
	Message string
}

func (e *FailError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s: %s", e.Message, e.Err)
		}
		return e.Err.Error()
	}
	return e.Message
}

func (e *FailError) Unwrap() error {
	return e.Err
}

type WarningError struct {
	// Message is a user-friendly string explaining the warning present
	Message string
}

func (e *WarningError) Error() string {
	return e.Message
}

func WrapSyncError(err error) error {
	switch syncErr := err.(type) {
	case *sync.NotInSyncError:
		return &RetryError{Err: syncErr}
	case *sync.UnrecoverableSyncError:
		return &FailError{Err: syncErr}
	case *sync.WarningError:
		return &WarningError{Message: syncErr.Error()}
	default:
		return err
	}
}
