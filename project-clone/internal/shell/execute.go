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

package shell

import (
	"fmt"
	"log"
	"os/exec"
)

type GitRefType int64

const (
	GitRefUnknown GitRefType = iota
	GitRefLocalBranch
	GitRefRemoteBranch
	GitRefTag
	GitRefHash
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

func GitSparseCloneProject(repoUrl, defaultRemoteName, destPath string) error {
	args := []string{
		"clone",
		"--sparse",
		repoUrl,
		"--origin", defaultRemoteName,
		"--",
		destPath,
	}
	return executeCommand("git", args...)
}

func GitSetupSparseCheckout(projectPath string, sparseCheckoutDir string) error {
	return executeCommand("git", "-C", projectPath, "sparse-checkout", "set", sparseCheckoutDir)
}

func GitFetchRemote(projectPath, remote string) error {
	return executeCommand("git", "-C", projectPath, "fetch", remote)
}

func GitCheckoutRef(projectPath, reference string) error {
	return executeCommand("git", "-C", projectPath, "checkout", reference)
}

func GitCheckoutBranch(projectPath, branchName, remote string) error {
	return executeCommand("git", "-C", projectPath, "checkout", "-b", branchName, "--track", fmt.Sprintf("%s/%s", remote, branchName))
}

func GitCheckoutBranchLocal(projectPath, branchName string) error {
	return executeCommand("git", "-C", projectPath, "checkout", branchName)
}

func GitSetTrackingRemoteBranch(projectPath, branchName, remote string) error {
	return executeCommand("git", "-C", projectPath, "branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remote, branchName), branchName)
}

// GitResolveReference determines if the provided revision is a (local) branch, tag, or hash for use when preparing a
// cloned repository. This is done by using `git show-ref` for branches/tags and `git rev-parse` for checking whether
// a commit hash exists. If the reference type cannot be determined, GitRefUnknown is returned.
func GitResolveReference(projectPath, remote, revision string) (GitRefType, error) {
	if err := executeCommandSilent("git", "-C", projectPath, "show-ref", "-q", "--verify", fmt.Sprintf("refs/heads/%s", revision)); err == nil {
		return GitRefLocalBranch, nil
	}
	if err := executeCommandSilent("git", "-C", projectPath, "show-ref", "-q", "--verify", fmt.Sprintf("refs/remotes/%s/%s", remote, revision)); err == nil {
		return GitRefRemoteBranch, nil
	}
	if err := executeCommandSilent("git", "-C", projectPath, "show-ref", "-q", "--verify", fmt.Sprintf("refs/tags/%s", revision)); err == nil {
		return GitRefTag, nil
	}
	if err := executeCommandSilent("git", "-C", projectPath, "rev-parse", "-q", "--verify", fmt.Sprintf("%s^{commit}", revision)); err == nil {
		return GitRefHash, nil
	}
	return GitRefUnknown, nil
}

func GitInitSubmodules(projectPath string) error {
	return executeCommand("git", "-C", projectPath, "submodule", "update", "--init", "--recursive")
}

func executeCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	return cmd.Run()
}

func executeCommandSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
