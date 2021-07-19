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

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container-images")

var envRegexp = regexp.MustCompile(`\${(RELATED_IMAGE_.*)}`)

const (
	webTerminalToolingImageEnvVar  = "RELATED_IMAGE_web_terminal_tooling"
	webhookServerImageEnvVar       = "RELATED_IMAGE_devworkspace_webhook_server"
	kubeRBACProxyImageEnvVar       = "RELATED_IMAGE_kube_rbac_proxy"
	pvcCleanupJobImageEnvVar       = "RELATED_IMAGE_pvc_cleanup_job"
	asyncStorageServerImageEnvVar  = "RELATED_IMAGE_async_storage_server"
	asyncStorageSidecarImageEnvVar = "RELATED_IMAGE_async_storage_sidecar"
	projectCloneImageEnvVar        = "RELATED_IMAGE_project_clone"
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

// GetKubeRBACProxyImage returns the image reference for the kube RBAC proxy. Returns
// the empty string if environment variable RELATED_IMAGE_kube_rbac_proxy is not defined
func GetKubeRBACProxyImage() string {
	val, ok := os.LookupEnv(kubeRBACProxyImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", kubeRBACProxyImageEnvVar), "Could not get webhook server image")
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

// GetPVCCleanupJobImage returns the image reference for the PVC cleanup job used to clean workspace
// files from the common PVC in a namespace.
func GetPVCCleanupJobImage() string {
	val, ok := os.LookupEnv(pvcCleanupJobImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", pvcCleanupJobImageEnvVar), "Could not get PVC cleanup job image")
		return ""
	}
	return val
}

func GetAsyncStorageServerImage() string {
	val, ok := os.LookupEnv(asyncStorageServerImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", asyncStorageServerImageEnvVar), "Could not get async storage server image")
		return ""
	}
	return val
}

func GetAsyncStorageSidecarImage() string {
	val, ok := os.LookupEnv(asyncStorageSidecarImageEnvVar)
	if !ok {
		log.Error(fmt.Errorf("environment variable %s is not set", asyncStorageSidecarImageEnvVar), "Could not get async storage sidecar image")
		return ""
	}
	return val
}

func GetProjectClonerImage() string {
	val, ok := os.LookupEnv(projectCloneImageEnvVar)
	if !ok {
		log.Info(fmt.Sprintf("Could not get initial project clone image: environment variable %s is not set", projectCloneImageEnvVar))
		return ""
	}
	return val
}

// FillPluginEnvVars replaces plugin devworkspaceTemplate .spec.components[].container.image environment
// variables of the form ${RELATED_IMAGE_*} with values from environment variables with the same name.
//
// Returns error if any referenced environment variable is undefined.
func FillPluginEnvVars(pluginDWT *dw.DevWorkspaceTemplate) (*dw.DevWorkspaceTemplate, error) {
	for idx, component := range pluginDWT.Spec.Components {
		if component.Container == nil {
			continue
		}
		img, err := getImageForEnvVar(component.Container.Image)
		if err != nil {
			return nil, err
		}
		pluginDWT.Spec.Components[idx].Container.Image = img
	}
	return pluginDWT, nil
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
