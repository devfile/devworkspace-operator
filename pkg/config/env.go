//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

import (
	"fmt"
	"os"
	"strconv"
)

type ControllerEnv struct{}

const (
	webhooksSecretNameEnvVar = "WEBHOOK_SECRET_NAME"
	developmentModeEnvVar    = "DEVELOPMENT_MODE"
	maxConcurrentReconciles  = "MAX_CONCURRENT_RECONCILES"
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
