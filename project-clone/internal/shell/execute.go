//
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

func GitFetchRemote(projectPath, remote string) error {
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
	return executeCommand("git", "fetch", remote)
}

func GitCheckoutRef(projectPath, reference string) error {
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
	return executeCommand("git", "checkout", reference)
}

func GitCheckoutBranch(projectPath, branchName, remote string) error {
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
	return executeCommand("git", "checkout", "-b", branchName, "--track", fmt.Sprintf("%s/%s", remote, branchName))
}

func GitCheckoutBranchLocal(projectPath, branchName string) error {
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
	return executeCommand("git", "checkout", branchName)
}

func GitSetTrackingRemoteBranch(projectPath, branchName, remote string) error {
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
	return executeCommand("git", "branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remote, branchName), branchName)
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
