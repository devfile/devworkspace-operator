//
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
//

package metrics

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
)

type FailureReason string

const (
	ReasonBadRequest             FailureReason = "BadRequest"
	ReasonInfrastructureFailure  FailureReason = "InfrastructureFailure"
	ReasonWorkspaceEngineFailure FailureReason = "WorkspaceEngineFailure"
	ReasonUnknown                FailureReason = "Unknown"
)

var devworkspaceFailureReasons = []FailureReason{
	ReasonBadRequest,
	ReasonInfrastructureFailure,
	ReasonWorkspaceEngineFailure,
	ReasonUnknown,
}

// GetFailureReason returns the FailureReason of the provided DevWorkspace
func GetFailureReason(wksp *common.DevWorkspaceWithConfig) FailureReason {
	failedCondition := conditions.GetConditionByType(wksp.Status.Conditions, dw.DevWorkspaceFailedStart)
	if failedCondition != nil {
		for _, reason := range devworkspaceFailureReasons {
			if failedCondition.Reason == string(reason) {
				return reason
			}
		}
	}
	return ReasonUnknown
}
