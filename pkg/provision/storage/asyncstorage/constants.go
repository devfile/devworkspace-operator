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

package asyncstorage

const (
	rsyncPort                 = 2222
	asyncServerServiceName    = "async-storage"
	asyncServerDeploymentName = "async-storage"
	asyncSecretVolumeName     = "async-storage-ssh"
	asyncSidecarContainerName = "async-storage-sidecar"

	asyncSidecarMemoryRequest = "64Mi"
	asyncSidecarMemoryLimit   = "512Mi"
	asyncServerMemoryRequest  = "256Mi"
	asyncServerMemoryLimit    = "512Mi"

	asyncStorageFinalizer = "controller.devfile.io/async-storage"
)

var asyncServerLabels = map[string]string{
	"app.kubernetes.io/name":                "async-storage",
	"app.kubernetes.io/part-of":             "devworkspace-operator",
	"controller.devfile.io/devworkspace_id": "",
}
