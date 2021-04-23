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

package internal

import (
	"log"
	"os"

	"github.com/devfile/devworkspace-operator/pkg/library/constants"
)

var (
	projectsRoot string
)

func init() {
	projectsRoot = os.Getenv(constants.ProjectsRootEnvVar)
	if projectsRoot == "" {
		log.Printf("Required environment variable %s is unset", constants.ProjectsRootEnvVar)
		os.Exit(1)
	}
}
