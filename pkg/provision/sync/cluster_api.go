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

package sync

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterAPI struct {
	Client           crclient.Client
	NonCachingClient crclient.Client
	Scheme           *runtime.Scheme
	Logger           logr.Logger
	Ctx              context.Context
}

// NotInSyncError is returned when a spec object is out-of-sync with its cluster counterpart
type NotInSyncError struct {
	Reason NotInSyncReason
	Object crclient.Object
}

type NotInSyncReason string

const (
	UpdatedObjectReason NotInSyncReason = "Updated object"
	CreatedObjectReason NotInSyncReason = "Created object"
	DeletedObjectReason NotInSyncReason = "Deleted object"
	NeedRetryReason     NotInSyncReason = "Need to retry"
)

func (e *NotInSyncError) Error() string {
	return fmt.Sprintf("%s %s is not ready: %s", reflect.TypeOf(e.Object).Elem().String(), e.Object.GetName(), e.Reason)
}

// NewNotInSync wraps creation of NotInSyncErrors for simplicity
func NewNotInSync(obj crclient.Object, reason NotInSyncReason) *NotInSyncError {
	return &NotInSyncError{
		Reason: reason,
		Object: obj,
	}
}

// UnrecoverableSyncError is returned when provided objects cannot be synced with the cluster due to
// an unexpected error (e.g. they are invalid according to the object's spec).
type UnrecoverableSyncError struct {
	Cause error
}

func (e *UnrecoverableSyncError) Error() string {
	return e.Cause.Error()
}

func (e *UnrecoverableSyncError) Unwrap() error {
	return e.Cause
}

// WarningError is returned when syncing is successful and can continue but there is a warning
// regarding the objects synced to the cluster (e.g. they will not be updated)
type WarningError struct {
	Message string
	Err     error
}

func (e *WarningError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err)
	}
	return e.Message
}

func (e *WarningError) Unwrap() error {
	return e.Err
}
