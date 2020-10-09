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

// Package images is intended to support deploying the operator on restricted networks. It contains
// utilities for translating images referenced by environment variables to regular image references,
// allowing images that are defined by a tag to be replaced by digests automatically. This allows all
// images used by the controller to be defined as environment variables on the controller deployment.
//
// All images defined must be referenced by an environment variable of the form RELATED_IMAGE_<name>.
// Functions in this package can be called to replace references to ${RELATED_IMAGE_<name>} with the
// corresponding environment variable.
package images

import (
	"fmt"
	"os"
	"regexp"

	"github.com/eclipse/che-plugin-broker/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container-images")

var envRegexp = regexp.MustCompile(`\${(RELATED_IMAGE_.*)}`)

const (
	webTerminalToolingImageEnvVar       = "RELATED_IMAGE_web_terminal_tooling"
	openshiftOAuthProxyImageEnvVar      = "RELATED_IMAGE_openshift_oauth_proxy"
	webhookServerImageEnvVar            = "RELATED_IMAGE_devworkspace_webhook_server"
	webhookKubernetesCertJobImageEnvVar = "RELATED_IMAGE_default_tls_secrets_creation_job"
)

// GetWebTerminalToolingImage returns the image reference for the webhook server image. Returns
// the empty string if environment variable RELATED_IMAGE_devworkspace_webhook_server is not defined
func GetWebhookServerImage() string {
	val, ok := os.LookupEnv(webhookServerImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", webhookServerImageEnvVar), "Could not get webhook server image")
		return ""
	}
	return val
}

// GetWebTerminalToolingImage returns the image reference for the default web tooling image. Returns
// the empty string if environment variable RELATED_IMAGE_web_terminal_tooling is not defined
func GetWebTerminalToolingImage() string {
	val, ok := os.LookupEnv(webTerminalToolingImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", webTerminalToolingImageEnvVar), "Could not get web terminal tooling image")
		return ""
	}
	return val
}

// GetOpenShiftOAuthProxyImage returns the image reference for the openshift OAuth proxy image, used
// for openshift-oauth workspace routingClass. Returns empty string if env var RELATED_IMAGE_openshift_oauth_proxy
// is not defined.
func GetOpenShiftOAuthProxyImage() string {
	val, ok := os.LookupEnv(openshiftOAuthProxyImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", openshiftOAuthProxyImageEnvVar), "Could not get OpenShift OAuth proxy image")
		return ""
	}
	return val
}

// GetWebhookCertJobImage returns the image reference for the webhook cert job image. Returns
// the empty string if environment variable RELATED_IMAGE_default_tls_secrets_creation_job is not defined
func GetWebhookCertJobImage() string {
	val, ok := os.LookupEnv(webhookKubernetesCertJobImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", webhookKubernetesCertJobImageEnvVar), "Could not get webhook cert job image")
		return ""
	}
	return val
}

// FillPluginMetaEnvVars replaces plugin meta .spec.Containers[].image and .spec.InitContainers[].image environment
// variables of the form ${RELATED_IMAGE_*} with values from environment variables with the same name.
//
// Returns error if any referenced environment variable is undefined.
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
