# DevWorkspace Operator release process

This document contains the instructions for the DevWorkspace Operator release procedure.

## GitHub Action

The release process is powered by the 'Release DevWorkspace Operator' [GitHub Action](https://github.com/devfile/devworkspace-operator/actions/workflows/release.yml).

In order to release a new version of the DevWorkspace Operator:

1. Trigger the prerelease by running the action using the workflow from the `main` branch. Provide the version in format `v0.y.z` e.g. `v0.13.0` and set `true` for the prelease field:

![Prerelease](prerelease.png?raw=true "Prerelease")

The action will create the dedicated release branch e.g. `0.13.x` and the prelease commit e.g https://github.com/devfile/devworkspace-operator/commit/4905c6c695e4d4945a3810df9a46a5ecf11d09f1 

> :warning: If necessary, cherry-pick any additional fixes to the release branch.

Build the image from the `0.13.x` branch and run manually a test with DevWorkspace startup against this version of the operator. If the workspace is started without errors you can proceed with the release.

2. Trigger the release by running the same action using the workflow from the branch that was created during the previous step e.g. `0.13.x`. Provide the version in format `v0.y.z` e.g. `v0.13.0` and set `false` for the prelease field:

![Release](release.png?raw=true "Release")

The action will create the release commit with the new OLM bundle e.g. https://github.com/devfile/devworkspace-operator/commit/aaa430987417c980001c7ae19932f78991fe9707, add the relevant tag in the repository e.g. `v0.13.0`, and push the images with the same tag to quay.io:

- https://quay.io/repository/devfile/devworkspace-controller?tag=v0.13.0&tab=tags
- https://quay.io/repository/devfile/devworkspace-operator-bundle?tag=v0.13.0&tab=tags
- https://quay.io/repository/devfile/project-clone?tag=v0.13.0&tab=tags

The index image with the `release` tag is also pushed automatically to quay.io:

- https://quay.io/repository/devfile/devworkspace-operator-index?tag=release&tab=tags
