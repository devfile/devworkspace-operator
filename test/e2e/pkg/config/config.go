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

import "github.com/devfile/devworkspace-operator/test/e2e/pkg/client"

var OperatorNamespace string
var DevWorkspaceNamespace string

var DevK8sClient, AdminK8sClient *client.K8sClient
