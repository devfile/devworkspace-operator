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

package version

var (
	// Version is the operator version
	Version = "v0.9.0+dev"
	// Commit is the commit hash corresponding to the code that was built. Can be suffixed with `-dirty`
	Commit string = "unknown"
	// BuildTime is the time of build of the binary
	BuildTime string = "unknown"
)
