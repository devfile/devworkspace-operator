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
)

var asyncServerLabels = map[string]string{
	"app.kubernetes.io/name":    "async-storage",
	"app.kubernetes.io/part-of": "devworkspace-operator",
}
