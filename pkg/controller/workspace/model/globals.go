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

package model

import (
	corev1 "k8s.io/api/core/v1"
)

var (
	DefaultApiEndpoint        = "http://localhost:9999/api"
	CheOriginalName           = "workspace"
	AuthEnabled               = "false"
	ServicePortProtocol       = corev1.ProtocolTCP
	ServiceAccount            = "che-workspace"
	SidecarDefaultMemoryLimit = "128M"
	PVCStorageSize            = "1Gi"
	CheVersion                = "7.1.0"
)
