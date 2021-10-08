#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
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
MAIN_BRANCH="main"
VERBOSE=""
TMP=""

usage () {
cat << EOF
This scripts handles upstream releasing stuff.

Prerelease is supposed to happen first, that will create .x
branch that will be used as a based for downstream DWO release.

After DWO release candidate bits are built, tested and pushed to prod
Release is supposed to happen, which will tag the DWO downstream release
base revision.

Arguments:
  --version     : version to release, v0.8.0
  --prerelease  : flag to perform prerelease. Is performed from main branch
  --release     : flag to perform release. Is performed from v0.x.y branch
  --revision    : [is not implemented yet] GIT_SHA of the target brach to be used for releasing

Development arguments to debug:
  --dry-run     : do not push changes locally
  --verbose     : enables verbose output
  --tmp-dir     : perform operation in repo cloned into temporary folder. Repo can be mofidied with DWO_REPO env var

Examples:
$0 --prerelease --version v0.1.0
$0 --release --version v0.1.0 --revision ${GIT_SHA}

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
      '--revision') echo "[ERROR] --revision is not implemented yet"; exit 1;;
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
  VERSION_CSV="${VERSION%%+*}"

  sed -i version/version.go -e "s#Version = \".*\"#Version = \"${VERSION_GO}\"#g"
  sed -i deploy/templates/components/csv/clusterserviceversion.yaml -r -e "s#(name: devworkspace-operator.)(v[0-9.]+)#\1v${VERSION_CSV}#g"
  sed -i deploy/templates/components/csv/clusterserviceversion.yaml -r -e "s#(version: )([0-9.]+)#\1${VERSION_CSV}#g"

  make generate manifests generate_default_deployment generate_olm_bundle_yaml
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
  local CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

  git add -A
  if [[ ! -z $(git status -s) ]]; then # dirty
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
    lastCommitComment="$(git log -1 --pretty=%B)"
    $DRY_RUN hub pull-request -f -m "${lastCommitComment}" -b "${CURRENT_BRANCH}" -h "${PR_BRANCH}"
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
  git checkout -b "${X_BRANCH}"

  echo "[INFO] Updating version to $VERSION"
  update_version $VERSION

  git_commit_and_push "[prerelease] Prepare branch for release" "ci-prerelease-$VERSION"

  # bump version in MAIN_BRANCH to next dev version
  [[ $X_BRANCH =~ ^([0-9]+)\.([0-9]+)\.x ]] \
    && BASE=${BASH_REMATCH[1]}; \
    NEXT=${BASH_REMATCH[2]}; \
    (( NEXT=NEXT+1 )) # for X_BRANCH=0.1.x, get BASE=0, NEXT=2

  NEXT_DEV_VERSION="${BASE}.${NEXT}.0+dev"
  git checkout ${MAIN_BRANCH}
  update_version $NEXT_DEV_VERSION
  git_commit_and_push "[release] Bump to ${NEXT_DEV_VERSION} in $MAIN_BRANCH" "ci-bump-$MAIN_BRANCH-$NEXT_DEV_VERSION"

  echo "[INFO] Prerelease is done"
}

# Perform release process for new version. Assumes that pre-release has been completed:
#
# Args:
#    $1 - Version to release
release() {
  local VERSION=$1

  echo "[INFO] Starting Release procedure"
  # derive bugfix branch from version
  X_BRANCH=${VERSION#v}
  X_BRANCH=${X_BRANCH%.*}.x

  git fetch origin "${X_BRANCH}:${X_BRANCH}" || true
  git checkout "${X_BRANCH}"

  # change image tag in Makefile
  DWO_QUAY_IMG="${DWO_QUAY_REPO}:${VERSION}"
  sed -i Makefile -r -e "s#quay.io/devfile/devworkspace-controller:[0-9a-zA-Z._-]+#${DWO_QUAY_IMG}#g"
  docker build -t "${DWO_QUAY_IMG}" -f ./build/Dockerfile .
  $DRY_RUN docker push "${DWO_QUAY_IMG}"

  PROJECT_CLONE_QUAY_IMG="${PROJECT_CLONE_QUAY_REPO}:${VERSION}"
  sed -i Makefile -r -e "s#quay.io/devfile/project-clone:[0-9a-zA-Z._-]+#${PROJECT_CLONE_QUAY_IMG}#g"
  docker build -t "${PROJECT_CLONE_QUAY_IMG}" -f ./project-clone/Dockerfile .
  $DRY_RUN docker push "${PROJECT_CLONE_QUAY_IMG}"

  export local DEFAULT_DWO_IMG="$DWO_QUAY_IMG"
  export local PROJECT_CLONE_IMG="$PROJECT_CLONE_QUAY_IMG"
  make generate manifests generate_default_deployment generate_olm_bundle_yaml

  # tag the release if the version/version.go file has changed
  git_commit_and_push "[release] Release ${VERSION}" "ci-release-version-$VERSION"
  git tag "${VERSION}"
  $DRY_RUN git push origin "${VERSION}"

  # now update ${X_BRANCH} to the new rc version
  git checkout "${X_BRANCH}"

  # bump the z digit
  [[ ${VERSION#v} =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]] \
    && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}";  \
    NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=0.1.2, get BASE=0.1, NEXT=3
  NEXT_VERSION_Z="${BASE}.${NEXT}"
  update_version "${NEXT_VERSION_Z}"
  git_commit_and_push "[release] Bump to ${NEXT_VERSION_Z} in $X_BRANCH" "ci-bump-$X_BRANCH-$NEXT_VERSION_Z"

  echo "[INFO] Release is done"
}

parse_args "$@"

[[ ! -z $VERBOSE ]] && echo "[INFO] Enabling verbose output" && set -x

# work in tmp dir
if [[ $TMP ]] && [[ -d $TMP ]]; then
  pushd "$TMP" > /dev/null || exit 1
  echo "[INFO] Check out ${DWO_REPO} to ${TMP}/${DWO_REPO##*/}"
  git clone "${DWO_REPO}" -q
  cd "${DWO_REPO##*/}" || exit 1
fi

[[ ! -z "$DO_PRERELEASE" ]] && prerelease $VERSION

[[ ! -z "$DO_RELEASE" ]] && release $VERSION

# cleanup tmp dir
if [[ $TMP ]] && [[ -d $TMP ]]; then
  popd > /dev/null || exit
  $DRY_RUN rm -fr "$TMP"
fi
