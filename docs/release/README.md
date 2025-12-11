# DevWorkspace Operator release process

This document contains the instructions for the DevWorkspace Operator release procedure.

## Preparing for a release

Prior to starting the release process, issues which have been resolved from pull requests need to be added to the milestone for the version being released.

### Creating a milestone

If necessary, create a milestone for the version being released. This is done by clicking "New milestone" on the [milestones page](https://github.com/devfile/devworkspace-operator/milestones). Set the title of the milestone in the format `v0.y.x` e.g. `v0.26.x`. Bug-fix releases do not need their own milestone. The due date and description fields can be left blank.

### Adding resolved issues to the milestone

Find all issues which have been resolved from pull requests since the last release, and add them to the milestone for the version that will be released. 

This can be done by using the following issue search filter: `is:issue sort:updated-desc is:closed closed:>={LAST_RELEASE_DATE} no:milestone`, where `{LAST_RELEASE_DATE}` is the date of the last commit that bumped the DevWorkspace Operator version. 

For example, when adding issues to the v0.26.x milestone, the search filter `is:issue sort:updated-desc is:closed closed:>=2024-01-17 no:milestone` should be used, as the version bump [commit](https://github.com/devfile/devworkspace-operator/commit/935f2405206a28375d51c114742865256cd99c75) occured on January 17th 2024.

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
export PROJECT_BACKUP_IMG=quay.io/yourrepo/project-backup:prerelease
# build and push project clone image
podman build -t "$PROJECT_CLONE_IMG" -f ./project-clone/Dockerfile .
podman push "$PROJECT_CLONE_IMG"
# build and push project backup image
podman build -t "$PROJECT_BACKUP_IMG" -f ./project-backup/Containerfile ./project-backup/
podman push "$PROJECT_BACKUP_IMG"
# build and push DevWorkspace Operator image
export DOCKER=podman # optional
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

### Post-release

After releasing a new version of the DevWorkspace Operator, it is necessary to copy the bundle files generated for that release as well as the new channel entries to the main branch by opening a PR.

#### Copying bundles & adding channel entries

On the release branch, 2 automated commits will be created: one pre-fixed with `[release]` and another pre-fixed with `[post-release]`. See [here](https://github.com/devfile/devworkspace-operator/commit/3c083e4840f2f26c5c8abd95c745d8f09666b586) for an example of the release commit, and [here](https://github.com/devfile/devworkspace-operator/commit/070adc49bcc34fe0d527d6a452a4c1aa01ee4a67) for an example of the post-release commit.

These commits will include changes to add an entry to the OLM catalog's `channel.yaml` and a generated bundle file. The `[release]` commit will modify the regular OLM catalog files, and the `[post-release]` commit will modify the OLM digest catalog files. These bundle and channel additions must be manually copied over from the release branch to the main branch in a PR to complete the release process. An examplary PR demonstrating this process can be found [here](https://github.com/devfile/devworkspace-operator/pull/1222).

### Creating a bugfix release

To create a bugfix release, the process is the same as [releasing](#releasing-a-new-version) above:

1. Cherry-pick any commits required for the bugfix release
2. Trigger the 'Release DevWorkspace Operator' action using the release branch (e.g. `0.13.x`), with `prerelease: false`
3. Open a PR to add the bundle files created for the new release to the main branch.
