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

package workspace

type ProvisioningStatus struct {
	// Continue should be true if cluster state matches spec state for this step
	Continue    bool
	Requeue     bool
	FailStartup bool
	Err         error
	Message     string
}

// Info returns the the user-friendly info about provisioning status
// It includes message or error or both if present
func (s *ProvisioningStatus) Info() string {
	var message = s.Message
	if s.Err != nil {
		if message != "" {
			message = message + ": " + s.Err.Error()
		} else {
			message = s.Err.Error()
		}
	}
	return message
}
