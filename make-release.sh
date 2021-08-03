#!/bin/bash
# Copyright (c) 2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
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
      '-v'|'--version') VERSION="$2"; shift 1;;
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

# bumps the version in version/version.go
bump_version () {
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

  NEXT_VERSION=$1
  BUMP_BRANCH=$2

  echo "[INFO] Bumping version ${BUMP_BRANCH} to ${NEXT_VERSION}"

  git checkout "${BUMP_BRANCH}"

  # change version/version.go file
  VERSION_GO="v${NEXT_VERSION}"
  if [[ "$BUMP_BRANCH" == "$MAIN_BRANCH" ]]; then
    VERSION_GO="$VERSION_GO+dev"
  fi
  sed -i version/version.go -e "s#Version = \".*\"#Version = \"${VERSION_GO}\"#g"
  sed -i deploy/templates/components/csv/clusterserviceversion.yaml -r -e "s#(name: devworkspace-operator.)(v[0-9.]+)#\1v${NEXT_VERSION}#g"
  sed -i deploy/templates/components/csv/clusterserviceversion.yaml -r -e "s#(version: )([0-9.]+)#\1${NEXT_VERSION}#g"
  git add -A
  if [[ ! -z $(git status -s) ]]; then # dirty
    COMMIT_MSG="[release] Bump to ${NEXT_VERSION} in ${BUMP_BRANCH}"
    git commit -m "${COMMIT_MSG}" --signoff
  fi
  git pull origin "${BUMP_BRANCH}"

  if [[ -z "$DRY_RUN" ]]; then
    set +e
    PUSH_TRY="$(git push origin "${BUMP_BRANCH}")"
    PUSH_TRY_EXIT_CODE=$?
    set -e
  else
    PUSH_TRY="protected branch hook declined"
  fi
  # shellcheck disable=SC2181
  if [[ "${PUSH_TRY_EXIT_CODE}" -gt 0 ]] || [[ $PUSH_TRY == *"protected branch hook declined"* ]]; then
    PR_BRANCH=pr-${BUMP_BRANCH}-to-${NEXT_VERSION}
    # create pull request for the main branch branch, as branch is restricted
    git branch "${PR_BRANCH}"
    git checkout "${PR_BRANCH}"
    git pull origin "${PR_BRANCH}" || true
    $DRY_RUN git push origin "${PR_BRANCH}"
    lastCommitComment="$(git log -1 --pretty=%B)"
    $DRY_RUN hub pull-request -f -m "${lastCommitComment}" -b "${BUMP_BRANCH}" -h "${PR_BRANCH}"
  fi
  git checkout "${CURRENT_BRANCH}"
}

prerelease() {
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
  sed -i version/version.go -e "s#Version = \".*\"#Version = \"${VERSION}\"#g"
  if [[ ! -z $(git status -s) ]]; then # dirty
    git add -A
    COMMIT_MSG="[prerelease] Remove dev from version"
    git commit -m "${COMMIT_MSG}" --signoff
  fi
  $DRY_RUN git push origin "${X_BRANCH}"

  # bump version in MAIN_BRANCH to next dev version
  [[ $X_BRANCH =~ ^([0-9]+)\.([0-9]+)\.x ]] \
    && BASE=${BASH_REMATCH[1]}; \
    NEXT=${BASH_REMATCH[2]}; \
    (( NEXT=NEXT+1 )) # for X_BRANCH=0.1.x, get BASE=0, NEXT=2

  NEXT_DEV_VERSION="${BASE}.${NEXT}.0"
  bump_version "${NEXT_DEV_VERSION}" "${MAIN_BRANCH}"
  echo "[INFO] Prerelease is done"
}

release() {
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

  bash -x ./deploy/generate-deployment.sh \
    --use-defaults \
    --default-image "${DWO_QUAY_IMG}" \
    --project-clone-image "${PROJECT_CLONE_QUAY_IMG}"

  # tag the release if the version/version.go file has changed
  if [[ ! -z $(git status -s) ]]; then # dirty
    COMMIT_MSG="[release] Release ${VERSION}"
    git add -A
    git commit -m "${COMMIT_MSG}" --signoff
  fi
  git tag "${VERSION}"
  $DRY_RUN git push origin "${VERSION}"

  # now update ${X_BRANCH} to the new rc version
  git checkout "${X_BRANCH}"

  # bump the z digit
  [[ ${VERSION#v} =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]] \
    && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}";  \
    NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=0.1.2, get BASE=0.1, NEXT=3
  NEXT_VERSION_Z="${BASE}.${NEXT}"
  bump_version "${NEXT_VERSION_Z}" "${X_BRANCH}"
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

[[ ! -z "$DO_PRERELEASE" ]] && prerelease

[[ ! -z "$DO_RELEASE" ]] && release

# cleanup tmp dir
if [[ $TMP ]] && [[ -d $TMP ]]; then
  popd > /dev/null || exit
  $DRY_RUN rm -fr "$TMP"
fi
