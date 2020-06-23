//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package images

import (
	"fmt"
	"github.com/eclipse/che-plugin-broker/model"
	"os"
	"regexp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container-images")

var envRegexp = regexp.MustCompile(`\${(.*)}`)

func FillPluginMetaEnvVars(pluginMeta model.PluginMeta) (model.PluginMeta, error) {
	for idx, container := range pluginMeta.Spec.Containers {
		img, err := getImageForEnvVar(container.Image)
		if err != nil {
			return model.PluginMeta{}, err
		}
		pluginMeta.Spec.Containers[idx].Image = img
	}
	for idx, initContainer := range pluginMeta.Spec.InitContainers {
		img, err := getImageForEnvVar(initContainer.Image)
		if err != nil {
			return model.PluginMeta{}, err
		}
		pluginMeta.Spec.InitContainers[idx].Image = img
	}
	return pluginMeta, nil
}

func isImageEnvVar(query string) bool {
	return envRegexp.MatchString(query)
}

func getImageForEnvVar(envStr string) (string, error) {
	if !isImageEnvVar(envStr) {
		// Value passed in is not env var, return unmodified
		return envStr, nil
	}
	matches := envRegexp.FindStringSubmatch(envStr)
	env := matches[1]
	val, ok := os.LookupEnv(env)
	if !ok {
		log.Info(fmt.Sprintf("Environment variable '%s' is unset. Cannot determine image to use", env))
		return "", fmt.Errorf("environment variable %s is unset", env)
	}
	return val, nil
}
