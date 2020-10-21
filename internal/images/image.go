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
	"strings"

	"github.com/devfile/devworkspace-operator/internal/cluster"

	"github.com/eclipse/che-plugin-broker/model"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container-images")

var envRegexp = regexp.MustCompile(`\${(RELATED_IMAGE_.*)}`)

const (
	webTerminalToolingImageEnvVar        = "RELATED_IMAGE_web_terminal_tooling"
	webTerminalToolingImageEnvVarDefault = "RELATED_IMAGE_web_terminal_tooling_default"
	openshiftOAuthProxyImageEnvVar       = "RELATED_IMAGE_openshift_oauth_proxy"
	webhookServerImageEnvVar             = "RELATED_IMAGE_devworkspace_webhook_server"
	webhookKubernetesCertJobImageEnvVar  = "RELATED_IMAGE_default_tls_secrets_creation_job"
)

// GetWebhookServerImage returns the image reference for the webhook server image. Returns
// the empty string if environment variable RELATED_IMAGE_devworkspace_webhook_server is not defined
func GetWebhookServerImage() string {
	val, ok := os.LookupEnv(webhookServerImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", webhookServerImageEnvVar), "Could not get webhook server image")
		return ""
	}
	return val
}

// GetWebTerminalToolingImage returns the image reference for the default web tooling image. It first looks at the
// environment variable RELATED_IMAGE_web_terminal_tooling. If that is not defined it will try to look for
// RELATED_IMAGE_web_terminal_tooling_default. On OpenShift the tooling image will be derived from the
// OpenShift version e.g. RELATED_IMAGE_web_terminal_tooling_4_5. If no environment variables are found or the cluster
// version could not be determined the empty string is returned.
func GetWebTerminalToolingImage() string {

	isOpenshift, err := cluster.IsOpenShift()
	if err != nil {
		log.Error(err, "Could not determine if the cluster was OpenShift")
		return ""
	}

	optionalOpenShiftVersion := ""
	if isOpenshift {
		version, err := cluster.OpenshiftVersion()
		if err != nil {
			log.Error(err, "Could not detect the OpenShift version")
			return ""
		}
		versions := strings.Split(version, ".")
		major := versions[0]
		minor := versions[1]
		optionalOpenShiftVersion = fmt.Sprintf("_%s_%s", major, minor)
	}

	val, ok := os.LookupEnv(webTerminalToolingImageEnvVar + optionalOpenShiftVersion)
	if !ok {
		val, ok := os.LookupEnv(webTerminalToolingImageEnvVarDefault)
		if !ok {
			log.Error(fmt.Errorf("environment variables %s and %s are not set", webTerminalToolingImageEnvVar, webTerminalToolingImageEnvVarDefault), "Could not get web terminal tooling image")
			return ""
		}
		return val
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
