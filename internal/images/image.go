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

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container-images")

const (
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

func GetProjectCloneImage() string {
	val, ok := os.LookupEnv(projectCloneImageEnvVar)
	if !ok {
		log.Info(fmt.Sprintf("Could not get initial project clone image: environment variable %s is not set", projectCloneImageEnvVar))
		return ""
	}
	return val
}
