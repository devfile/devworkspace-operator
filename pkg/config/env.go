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
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/api/resource"
)

type ControllerEnv struct{}

const (
	webhooksSecretNameEnvVar = "WEBHOOK_SECRET_NAME"
	developmentModeEnvVar    = "DEVELOPMENT_MODE"
	maxConcurrentReconciles  = "MAX_CONCURRENT_RECONCILES"

	WebhooksMemLimitEnvVar   = "WEBHOOKS_SERVER_MEMORY_LIMIT"
	WebhooksMemRequestEnvVar = "WEBHOOKS_SERVER_MEMORY_REQUEST"
	WebhooksCPULimitEnvVar   = "WEBHOOKS_SERVER_CPU_LIMIT"
	WebhooksCPURequestEnvVar = "WEBHOOKS_SERVER_CPU_REQUEST"
)

func GetWebhooksSecretName() (string, error) {
	env := os.Getenv(webhooksSecretNameEnvVar)
	if env == "" {
		return "", fmt.Errorf("environment variable %s is unset", webhooksSecretNameEnvVar)
	}
	return env, nil
}

func GetDevModeEnabled() bool {
	return os.Getenv(developmentModeEnvVar) == "true"
}

func GetMaxConcurrentReconciles() (int, error) {
	env := os.Getenv(maxConcurrentReconciles)
	if env == "" {
		return 0, fmt.Errorf("environment variable %s is unset", maxConcurrentReconciles)
	}
	val, err := strconv.Atoi(env)
	if err != nil {
		return 0, fmt.Errorf("could not parse environment variable %s: %s", maxConcurrentReconciles, err)
	}
	return val, nil
}

func GetResourceQuantityFromEnvVar(env string) (*resource.Quantity, error) {
	val := os.Getenv(env)
	if val == "" {
		return nil, fmt.Errorf("environment variable %s is unset", env)
	}
	quantity, err := resource.ParseQuantity(val)
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment variable %s: %s", env, err)
	}
	return &quantity, nil
}

func GetWorkspaceControllerSA() (string, error) {
	saName := os.Getenv(constants.ControllerServiceAccountNameEnvVar)
	if saName == "" {
		return "", fmt.Errorf("environment variable %s is unset", constants.ControllerServiceAccountNameEnvVar)
	}
	return saName, nil
}
