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

// Package storage contains library functions for provisioning volumes and volumeMounts in containers according to the
// volume components in a devfile. These functions also handle mounting project sources to containers that require it.
//
// TODO:
// - Add functionality for generating PVCs with the appropriate size based on size requests in the devfile
// - Devfile API spec is unclear on how mountSources should be handled -- mountPath is assumed to be /projects
//   and volume name is assumed to be "projects"
//   see issues:
//     - https://github.com/devfile/api/issues/290
//     - https://github.com/devfile/api/issues/291
package storage
