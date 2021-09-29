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
package infrastructure

import (
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// IsWebhookConfigurationEnabled returns true if both of mutating and validating webhook configurations are enabled
func IsWebhookConfigurationEnabled() (bool, error) {
	kubeCfg, err := config.GetConfig()
	if err != nil {
		return false, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeCfg)
	if err != nil {
		return false, err
	}
	_, apiResources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}

	if admissionRegistrationResources := findAPIResources(apiResources, "admissionregistration.k8s.io/v1"); admissionRegistrationResources != nil {
		isMutatingHookAvailable := false
		isValidatingMutatingHookAvailable := false
		for i := range admissionRegistrationResources {
			if admissionRegistrationResources[i].Name == "mutatingwebhookconfigurations" {
				isMutatingHookAvailable = true
			}

			if admissionRegistrationResources[i].Name == "validatingwebhookconfigurations" {
				isValidatingMutatingHookAvailable = true
			}
		}

		return isMutatingHookAvailable && isValidatingMutatingHookAvailable, nil
	}

	return false, nil
}
