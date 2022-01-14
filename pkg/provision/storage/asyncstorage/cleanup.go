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

package asyncstorage

import (
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

// RemoveAuthorizedKeyFromConfigMap removes the ssh key used by a given workspace from the common async storage
// authorized keys configmap.
func RemoveAuthorizedKeyFromConfigMap(workspace *dw.DevWorkspace, api sync.ClusterAPI) (retry bool, err error) {
	sshSecret, err := getSSHSidecarSecretCluster(workspace, api)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	pubkey, _, err := ExtractSSHKeyPairFromSecret(sshSecret)
	if err != nil {
		return false, err
	}

	configmap, err := getSSHAuthorizedKeysConfigMapCluster(workspace.Namespace, api)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	didChange, err := removeAuthorizedKeyFromConfigMap(configmap, pubkey)
	if err != nil {
		return false, err
	}
	if !didChange {
		return false, nil
	}

	err = api.Client.Update(api.Ctx, configmap)
	if err != nil {
		if k8sErrors.IsConflict(err) {
			return true, nil
		}
		return false, err
	}

	if coputil.HasFinalizer(sshSecret, asyncStorageFinalizer) {
		coputil.RemoveFinalizer(sshSecret, asyncStorageFinalizer)
		err := api.Client.Update(api.Ctx, sshSecret)
		if err != nil && !k8sErrors.IsConflict(err) {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
