#!/bin/bash
#
# Copyright (c) 2019-2025 Red Hat, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#


set -e
DWO_REPO="${DWO_REPO:-git@github.com:devfile/devworkspace-operator}"
DWO_QUAY_REPO="${DWO_QUAY_REPO:-quay.io/devfile/devworkspace-controller}"
PROJECT_CLONE_QUAY_REPO="${PROJECT_CLONE_QUAY_REPO:-quay.io/devfile/project-clone}"
DWO_BUNDLE_QUAY_REPO="${DWO_BUNDLE_QUAY_REPO:-quay.io/devfile/devworkspace-operator-bundle}"
DWO_INDEX_IMAGE="${DWO_INDEX_IMAGE:-quay.io/devfile/devworkspace-operator-index:release}"
DWO_DIGEST_INDEX_IMAGE="${DWO_DIGEST_INDEX_IMAGE:-quay.io/devfile/devworkspace-operator-index:release-digest}"
MAIN_BRANCH="main"
ARCHITECTURES="linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"
VERBOSE=""
TMP=""

usage () {
cat << EOF
This scripts handles the upstream release process for DWO.

To begin a new release process, use the '--prerelease' flag. This will create a
branch, e.g. '0.<VERSION>.x' for the specified version and update all relevant
files within that branch to reflect the new version. This branch should then be
tested, with additional commits cherry-picked as necessary to prepare for
release. Once the HEAD commit of the '0.<VERSION>.x' branch is ready for
release, use the '--release' from the release branch to create the release tag
and build and push release container images to the Quay repo. Running with the
release flag will also update versions in the release branch to reflect the next
bugfix release (e.g. will update from v0.8.0 to v0.8.1).

To issue a bugfix release, cherry-pick commits into the existing release branch
as necessary and run this script with the '--release' flag for the given bugfix
version.

Arguments:
  --version     : version to release, e.g. v0.8.0
  --prerelease  : flag to perform prerelease. Is performed from main branch
  --release     : flag to perform release. Is performed from v0.x.y branch

Development arguments to debug:
  --dry-run     : do not push changes locally
  --verbose     : enables verbose output
  --tmp-dir     : perform operation in repo cloned into temporary folder. Repo can be modified with DWO_REPO env var

Examples:
$0 --prerelease --version v0.1.0
$0 --release --version v0.1.0

This script is intended to be triggered by GitHub Actions on the repo.
EOF
}

dryrun() {
    printf -v cmd_str '%q ' "$@"; echo "DRYRUN: Not executing $cmd_str" >&2
}

parse_args() {
  while [[ "$#" -gt 0 ]]; do
    case $1 in
      # Ensure version parameter is in format vX.Y.Z
      '-v'|'--version') VERSION="v${2#v}"; shift 1;;
      '--tmp-dir') TMP=$(mktemp -d); shift 0;;
      '--release') DO_RELEASE=true; shift 0;;
      '--prerelease') DO_PRERELEASE='true'; shift 0;;
      '--dry-run') DRY_RUN='dryrun'; shift 0;;
      '--verbose') VERBOSE='true'; shift 0;;
      '--help') usage; exit 0;;
      *) echo "[ERROR] Unknown parameter is used: $1."; usage; exit 1;;
    esac
    shift 1
  done

  if [ -z "${VERSION}" ]; then
    echo "[ERROR] Required parameter --version is missing."
    usage
    exit 1
  fi

  version_pattern='^v[0-9]+\.[0-9]+\.[0-9]+$'
  if [[ ! ${VERSION} =~ $version_pattern ]]; then
    echo "did not match"
    echo "exit 1"
  fi
}

# Updates hard-coded version strings in repo (version.go, CSV templates). For go files, version is prepended with 'v',
# e.g. if version is 0.10.0, Go files use version v0.10.0. Files are regenerated but changes are not committed to the
# repo.
# Args:
#    $1 - Version to set in files
update_version() {
  local VERSION=${1#v}

  # change version/version.go file
  VERSION_GO="v${VERSION}"
  VERSION_CSV="$VERSION"

  sed -i version/version.go -e "s#Version = \".*\"#Version = \"${VERSION_GO}\"#g"
  yq -Yi \
    --arg operator_name "devworkspace-operator.v$VERSION_CSV" \
    --arg version "$VERSION_CSV" \
    '.metadata.name = $operator_name | .spec.version = $version' deploy/templates/components/csv/clusterserviceversion.yaml

  make generate_all
}

# Updates container images and tags used in deployment templates for a release version
# of DWO. Sets appropriate image names for controller and project clone images and
# updates defaults in Makefile. Does not commit changes to repo.
# Args:
#   $1 - Version for images
update_images() {
  VERSION="$1"
  # Get image tags
  DWO_QUAY_IMG="${DWO_QUAY_REPO}:${VERSION}"
  PROJECT_CLONE_QUAY_IMG="${PROJECT_CLONE_QUAY_REPO}:${VERSION}"
  DWO_BUNDLE_QUAY_IMG="${DWO_BUNDLE_QUAY_REPO}:${VERSION}"

  # Update defaults in Makefile
  sed -i Makefile -r \
    -e "s|quay.io/devfile/devworkspace-controller:[0-9a-zA-Z._-]+|${DWO_QUAY_IMG}|g" \
    -e "s|quay.io/devfile/project-clone:[0-9a-zA-Z._-]+|${PROJECT_CLONE_QUAY_IMG}|g" \
    -e "s|quay.io/devfile/devworkspace-operator-bundle:[0-9a-zA-Z._-]+|${DWO_BUNDLE_QUAY_IMG}|g" \
    -e "s|quay.io/devfile/devworkspace-operator-index:[0-9a-zA-Z._-]+|${DWO_INDEX_IMAGE}|g"

  # Update defaults in generate_deployment.sh
  sed -i build/scripts/generate_deployment.sh -r \
    -e "s|quay.io/devfile/devworkspace-controller:[0-9a-zA-Z._-]+|${DWO_QUAY_IMG}|g" \
    -e "s|quay.io/devfile/project-clone:[0-9a-zA-Z._-]+|${PROJECT_CLONE_QUAY_IMG}|g"

  local DEFAULT_DWO_IMG="$DWO_QUAY_IMG"
  local PROJECT_CLONE_IMG="$PROJECT_CLONE_QUAY_IMG"

  export DEFAULT_DWO_IMG
  export PROJECT_CLONE_IMG
  make generate_all
}

# Build and push images for specified release version. Respects the DRY_RUN flag
# TODO:
#   - Build release images for bundle and index
# Args:
#   $1 - Version for images
build_and_push_images() {
  DWO_QUAY_IMG="${DWO_QUAY_REPO}:${VERSION}"
  PROJECT_CLONE_QUAY_IMG="${PROJECT_CLONE_QUAY_REPO}:${VERSION}"

  if [ "$DRY_RUN" == "dryrun" ]; then
    docker buildx build . -t "${DWO_QUAY_IMG}" -f ./build/Dockerfile \
    --platform "$ARCHITECTURES"
    docker buildx build . -t "${PROJECT_CLONE_QUAY_IMG}" -f ./project-clone/Dockerfile \
    --platform "$ARCHITECTURES"
  else
    docker buildx build . -t "${DWO_QUAY_IMG}" -f ./build/Dockerfile \
    --platform "$ARCHITECTURES" \
    --push
    docker buildx build . -t "${PROJECT_CLONE_QUAY_IMG}" -f ./project-clone/Dockerfile \
    --platform "$ARCHITECTURES" \
    --push
  fi
}

# Commit and push changes in local repo to remote (respecting DRY_RUN setting). If the branch cannot be pushed to,
# create a temporary branch and open a PR based off it. If repo state is clean, no new commit is created and branch
# is pushed.
# Args:
#    $1 - Commit message to use for commit
#    $2 - PR branch name to use if necessary
git_commit_and_push() {
  local COMMIT_MSG="$1"
  local PR_BRANCH="$2"
  local CURRENT_BRANCH
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

  git add -A
  if [[ -n $(git status -s) ]]; then # dirty
    git commit -m "${COMMIT_MSG}" --signoff
  fi

  if [[ -z "$DRY_RUN" ]]; then
    set +e
    PUSH_TRY="$(git push origin "${CURRENT_BRANCH}")"
    PUSH_TRY_EXIT_CODE=$?
    set -e
  else
    PUSH_TRY="protected branch hook declined"
  fi
  # shellcheck disable=SC2181
  if [[ "${PUSH_TRY_EXIT_CODE}" -gt 0 ]] || [[ $PUSH_TRY == *"protected branch hook declined"* ]]; then
    # create pull request for the main branch branch, as branch is restricted
    git branch "${PR_BRANCH}"
    git checkout "${PR_BRANCH}"
    git pull origin "${PR_BRANCH}" || true
    $DRY_RUN git push origin "${PR_BRANCH}"
    $DRY_RUN gh pr create -f -B "${CURRENT_BRANCH}" -H "${PR_BRANCH}"
  fi
  git checkout "${CURRENT_BRANCH}"
}

# Perform prerelease actions in repo based on VERSION env var (--version flag):
#   1. Create a new release branch, e.g v0.10.x if $VERSION is v0.10.0
#   2. Update version in release branch to reflect $VERSION
#   3. Push release branch to remote repo
#   4. Update main branch to reflect next version
#   5. Push changes to remote main branch (via PR if necessary)
# Args:
#   $1 - Next version to release (e.g. v0.10.0). Should be minor release, not bugfix
prerelease() {
  local VERSION=$1

  if [ "${VERSION##*.}" != "0" ]; then
    echo "[ERROR] Flag --prerelease should not be used for bugfix versions"
    exit 1
  fi

  echo "[INFO] Starting prerelease procedure"
  # derive bugfix branch from version
  X_BRANCH=${VERSION#v}
  X_BRANCH=${X_BRANCH%.*}.x

  echo "[INFO] Creating ${X_BRANCH} from ${MAIN_BRANCH}"
  git checkout ${MAIN_BRANCH}
  git checkout "${X_BRANCH}"

  echo "[INFO] Updating version to $VERSION"
  update_version "$VERSION"
  update_images "$VERSION"

  git_commit_and_push "[prerelease] Prepare branch for release" "ci-prerelease-$VERSION"

  # bump version in MAIN_BRANCH to next dev version
  [[ $X_BRANCH =~ ^([0-9]+)\.([0-9]+)\.x ]] \
    && BASE=${BASH_REMATCH[1]}; \
    NEXT=${BASH_REMATCH[2]}; \
    (( NEXT=NEXT+1 )) # for X_BRANCH=0.1.x, get BASE=0, NEXT=2

  NEXT_DEV_VERSION="${BASE}.${NEXT}.0-dev"
  git checkout ${MAIN_BRANCH}
  update_version "$NEXT_DEV_VERSION"
  git_commit_and_push "chore: release: bump to ${NEXT_DEV_VERSION} in $MAIN_BRANCH" "ci-bump-$MAIN_BRANCH-$NEXT_DEV_VERSION"

  echo "[INFO] Prerelease is done"
}

# Perform release process for new version. Assumes that pre-release has been completed:
#
# Args:
#    $1 - Version to release
release() {
  local VERSION=$1

  if git ls-remote --exit-code --tags origin "${VERSION}" > /dev/null; then
    echo "Version $VERSION is already tagged; aborting"
    exit 1
  fi

  echo "[INFO] Starting Release procedure"
  # derive bugfix branch from version
  X_BRANCH=${VERSION#v}
  X_BRANCH=${X_BRANCH%.*}.x

  git fetch origin "${X_BRANCH}:${X_BRANCH}" || true
  git checkout "${X_BRANCH}"

  # Build bundle and index images
  $DRY_RUN build/scripts/build_index_image.sh \
    --release \
    --bundle-tag "$VERSION" \
    --bundle-repo "$DWO_BUNDLE_QUAY_REPO" \
    --index-image "$DWO_INDEX_IMAGE" \
    --force

  # Commit changes from releasing bundle
  git_commit_and_push "[release] Add OLM bundle for $VERSION in $X_BRANCH" "ci-add-bundle-$VERSION"

  # Tag current commit as release version
  git tag "${VERSION}"
  $DRY_RUN git push origin "${VERSION}"

  # Build container images for relase
  build_and_push_images "$VERSION"

  $DRY_RUN build/scripts/build_digests_bundle.sh \
    --bundle "${DWO_BUNDLE_QUAY_REPO}:${VERSION}" \
    --render olm-catalog/release-digest/ \
    --push "${DWO_BUNDLE_QUAY_REPO}:${VERSION}-digest" \
    --container-tool docker \
    --debug

  CHANNEL_ENTRY_JSON=$(yq --arg version "$VERSION" '.entries[] | select(.name == "devworkspace-operator.\($version)")' olm-catalog/release/channel.yaml)
  yq -Y -i --argjson entry "$CHANNEL_ENTRY_JSON" '.entries |= . + [$entry]' olm-catalog/release-digest/channel.yaml

  opm validate olm-catalog/release-digest/
  echo "[INFO] Building index image $DWO_DIGEST_INDEX_IMAGE"
  docker build . -t "$DWO_DIGEST_INDEX_IMAGE" -f "build/index.release-digest.Dockerfile"
  docker push "$DWO_DIGEST_INDEX_IMAGE" 2>&1

  # Commit changes from rendering digests bundle
  git_commit_and_push "[post-release] Add OLM digest bundle for $VERSION in $X_BRANCH" "ci-add-digest-bundle-$VERSION"

  # Update ${X_BRANCH} to the new rc version
  git checkout "${X_BRANCH}"

  # bump the z digit
  [[ ${VERSION#v} =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]] \
    && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}";  \
    NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=0.1.2, get BASE=0.1, NEXT=3
  NEXT_VERSION_Z="v${BASE}.${NEXT}"

  update_version "$NEXT_VERSION_Z"
  update_images "$NEXT_VERSION_Z"
  git_commit_and_push "chore: release: bump to ${NEXT_VERSION_Z} in $X_BRANCH" "ci-bump-$X_BRANCH-$NEXT_VERSION_Z"

  echo "[INFO] Release is done"
}

parse_args "$@"

[[ -n $VERBOSE ]] && echo "[INFO] Enabling verbose output" && set -x

# work in tmp dir
if [[ $TMP ]] && [[ -d $TMP ]]; then
  pushd "$TMP" > /dev/null || exit 1
  echo "[INFO] Check out ${DWO_REPO} to ${TMP}/${DWO_REPO##*/}"
  git clone "${DWO_REPO}" -q
  cd "${DWO_REPO##*/}" || exit 1
fi

[[ -n "$DO_PRERELEASE" ]] && prerelease "$VERSION"

[[ -n "$DO_RELEASE" ]] && release "$VERSION"

# cleanup tmp dir
if [[ $TMP ]] && [[ -d $TMP ]]; then
  popd > /dev/null || exit
  $DRY_RUN rm -fr "$TMP"
fi
