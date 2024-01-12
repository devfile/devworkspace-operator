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
	"strings"
)

var badRequestFailures = []string{
	"CrashLoopBackOff",
	"ErrImagePull",
	"ImagePullBackOff",
}

var infrastructureFailures = []string{
	"CreateContainerError",
	"RunContainerError",
	"FailedScheduling",
	"FailedMount",
}

// DetermineProvisioningFailureReason scans a deployment provisioning status info message
// and returns the corresponding failure reason.
// If a failure reason cannot be found, an Unknown reason is returned.
func DetermineProvisioningFailureReason(statusMessage string) FailureReason {
	for _, failure := range badRequestFailures {
		if strings.Contains(statusMessage, failure) {
			return ReasonBadRequest
		}
	}
	for _, failure := range infrastructureFailures {
		if strings.Contains(statusMessage, failure) {
			return ReasonInfrastructureFailure
		}
	}
	return ReasonUnknown
}
