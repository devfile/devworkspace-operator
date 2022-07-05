# DevWorkspace Operator release process

This document contains the instructions for the DevWorkspace Operator release procedure.

## GitHub Action

The release process is powered by the 'Release DevWorkspace Operator' [GitHub Action](https://github.com/devfile/devworkspace-operator/actions/workflows/release.yml).

### Preparing release branch
Trigger the prerelease by running the 'Release DevWorkspace Operator' action using the workflow from the `main` branch. Provide the version in format `v0.y.z` e.g. `v0.13.0` and set `true` for the prelease field:

![Prerelease](prerelease.png?raw=true "Prerelease")

The action will create the dedicated release branch e.g. `0.13.x` and the prelease commit e.g https://github.com/devfile/devworkspace-operator/commit/4905c6c695e4d4945a3810df9a46a5ecf11d09f1 

This step is required for every new minor release of the DevWorkspace Operator.

### Testing prerelease artifacts

The release job updates the yaml files used to deploy the DevWorkspace Operator to use the new images. However, for commits that aren't tagged for release, these yaml files will refer to still-unbuilt container images. For example, commits in the `0.15.x` branch will use `quay.io/devfile/devworkspace-controller:v0.15.0`, but that image will only be pushed once the full release happens.

This means that to test commits in a release branch before running the release job, it's necessary to manually build all DevWorkspace Operator images and override them in the deployment:

```bash
export DWO_IMG=quay.io/yourrepo/devworkspace-controller:prerelease
export PROJECT_CLONE_IMG=quay.io/yourrepo/project-clone:prerelease
# build and push project clone image
podman build -t "$PROJECT_CLONE_IMG" -f ./project-clone/Dockerfile .
podman push "$PROJECT_CLONE_IMG"
# build and push DevWorkspace Operator image
make docker
# deploy DevWorkspace Operator using these images
make install
```

### Releasing a new version

> :warning: If necessary, cherry-pick any additional fixes to the release branch.

Trigger the release by running the 'Release DevWorkspace Operator' action using the workflow from the release branch that was created in the pre-release setup, e.g. `0.13.x`. Provide the version in format `v0.y.z`, e.g. `v0.13.0` and set `false` for the prelease field:

![Release](release.png?raw=true "Release")

The action will create the release commit with the new OLM bundle e.g. https://github.com/devfile/devworkspace-operator/commit/aaa430987417c980001c7ae19932f78991fe9707, add the relevant tag in the repository e.g. `v0.13.0`, and push the images with the same tag to quay.io:

- https://quay.io/repository/devfile/devworkspace-controller?tag=v0.13.0&tab=tags
- https://quay.io/repository/devfile/devworkspace-operator-bundle?tag=v0.13.0&tab=tags
- https://quay.io/repository/devfile/project-clone?tag=v0.13.0&tab=tags

The index image with the `release` tag is also pushed automatically to quay.io:

- https://quay.io/repository/devfile/devworkspace-operator-index?tag=release&tab=tags

After releasing a new version of the DevWorkspace Operator, it is necessary to copy the bundle files generated for that release to the main branch by opening a PR.

### Creating a bugfix release

To create a bugfix release, the process is the same as [releasing](#releasing-a-new-version) above:

1. Cherry-pick any commits required for the bugfix release
2. Trigger the 'Release DevWorkspace Operator' action using the release branch (e.g. `0.13.x`), with `prerelease: false`
3. Open a PR to add the bundle files created for the new release to the main branch.
