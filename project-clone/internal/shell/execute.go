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
	"fmt"
	"log"
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

// GitResetProject runs `git reset --hard` in the project specified by projectPath
func GitResetProject(projectPath string) error {
	currDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %s", err)
	}
	defer func() {
		if err := os.Chdir(currDir); err != nil {
			log.Printf("failed to return to original working directory: %s", err)
		}
	}()

	err = os.Chdir(projectPath)
	if err != nil {
		return fmt.Errorf("failed to move to project directory %s: %s", projectPath, err)
	}
	return executeCommand("git", "reset", "--hard")
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
