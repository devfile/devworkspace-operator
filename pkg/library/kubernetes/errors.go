// Copyright (c) 2019-2023 Red Hat, Inc.
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

package kubernetes

import "github.com/devfile/devworkspace-operator/pkg/provision/sync"

type RetryError struct {
	Err error
}

func (e *RetryError) Error() string {
	return e.Err.Error()
}

type FailError struct {
	Err error
}

func (e *FailError) Error() string {
	return e.Err.Error()
}

func wrapSyncError(err error) error {
	switch syncErr := err.(type) {
	case *sync.NotInSyncError:
		return &RetryError{syncErr}
	case *sync.UnrecoverableSyncError:
		return &FailError{syncErr}
	case *sync.WarningError:
		return &WarningError{syncErr}
	default:
		return err
	}
}

type WarningError struct {
	Err error
}

func (e *WarningError) Error() string {
	return e.Err.Error()
}
