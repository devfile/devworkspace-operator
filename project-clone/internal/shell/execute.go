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

package shell

import (
	"os"
	"os/exec"
)

// GitCloneProject constructs a command-line string for cloning a git project, and delegates execution
// to the os/exec package.
func GitCloneProject(repoUrl, defaultRemoteName, destPath string) error {
	args := []string{
		"clone",
		repoUrl,
		"--origin", defaultRemoteName,
		"--",
		destPath,
	}
	return executeCommand("git", args...)
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
