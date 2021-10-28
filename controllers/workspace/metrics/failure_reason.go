//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

type FailureReason struct {
	snakeCase string
	camelCase string
}

var (
	ReasonBadRequest             = FailureReason{snakeCase: "bad_request", camelCase: "BadRequest"}
	ReasonInfrastructureFailure  = FailureReason{snakeCase: "infrastructure_failure", camelCase: "InfrastructureFailure"}
	ReasonWorkspaceEngineFailure = FailureReason{snakeCase: "workspace_engine_failure", camelCase: "WorkspaceEngineFailure"}
	ReasonUnknown                = FailureReason{snakeCase: "unknown", camelCase: "Unknown"}
)

var devworkspaceFailureReasons [4]FailureReason = [4]FailureReason{
	ReasonBadRequest,
	ReasonInfrastructureFailure,
	ReasonWorkspaceEngineFailure,
	ReasonUnknown,
}

func (f *FailureReason) CamelCase() string {
	return f.camelCase
}

func (f *FailureReason) SnakeCase() string {
	return f.snakeCase
}

// GetFailureReasonFromStr returns the corresponding FailureReason given an input
// string representation. The input string representation can be snake case or camel case.
func GetFailureReasonFromStr(reason string) FailureReason {
	for _, v := range devworkspaceFailureReasons {
		if reason == v.snakeCase || reason == v.camelCase {
			return v
		}
	}
	return ReasonUnknown
}
