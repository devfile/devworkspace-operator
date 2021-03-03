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
#!/bin/bash	
QUAY_REPO="quay.io/devfile/devworkspace-controller:${VERSION}"	
MAIN_BRANCH="main"

while [[ "$#" -gt 0 ]]; do	
  case $1 in	
    '-v'|'--version') VERSION="$2"; shift 1;;		
  esac	
  shift 1	
done	

bump_version () {	
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)	

  NEXT_VERSION=$1	
  BUMP_BRANCH=$2	

  git checkout "${BUMP_BRANCH}"	

  echo "Updating project version to ${NEXT_VERSION}"	
  echo "${VERSION}" > VERSION 
  if [[ ${NOCOMMIT} -eq 0 ]]; then	
    COMMIT_MSG="[release] Bump to ${NEXT_VERSION} in ${BUMP_BRANCH}"	
    git commit -asm "${COMMIT_MSG}"	
    git pull origin "${BUMP_BRANCH}"	

    # set +e
    # PUSH_TRY="$(git push origin "${BUMP_BRANCH}")"	
    # # shellcheck disable=SC2181	
    # set -e
    # if [[ $? -gt 0 ]] || [[ $PUSH_TRY == *"protected branch hook declined"* ]]; then	
    #   PR_BRANCH=pr-${BUMP_BRANCH}-to-${NEXT_VERSION}	
    #   # create pull request for the main branch branch, as branch is restricted	
    #   git branch "${PR_BRANCH}"	
    #   git checkout "${PR_BRANCH}"	
    #   git pull origin "${PR_BRANCH}"	
    #   git push origin "${PR_BRANCH}"	
    #   lastCommitComment="$(git log -1 --pretty=%B)"	
    #   hub pull-request -f -m "${lastCommitComment}" -b "${BUMP_BRANCH}" -h "${PR_BRANCH}"	
    # fi 	
  fi	
  git checkout "${CURRENT_BRANCH}"	
}	

usage ()	
{	
  echo "Usage: $0 --version [VERSION TO RELEASE]"	
  echo "Example: $0 --version 0.1.0"; echo	
}	

if [[ ! ${VERSION} ]]; then	
  usage	
  exit 1	
fi	


# derive branch from version	
BRANCH=${VERSION%.*}.x	

# if doing a .0 release, use main branch; if doing a .z release, use $BRANCH	
if [[ ${VERSION} == *".0" ]]; then	
  BASEBRANCH="${MAIN_BRANCH}"	
else 	
  BASEBRANCH="${BRANCH}"	
fi	

# get sources from ${BASEBRANCH} branch	
git fetch origin "${BASEBRANCH}":"${BASEBRANCH}"	
git checkout "${BASEBRANCH}"	

# create new branch off ${BASEBRANCH} (or check out latest commits if branch already exists), then push to origin
if [[ "${BASEBRANCH}" != "${BRANCH}" ]]; then
  git branch "${BRANCH}" || git checkout "${BRANCH}" && git pull origin "${BRANCH}"
#   git push origin "${BRANCH}"
  git fetch origin "${BRANCH}:${BRANCH}" || true
  git checkout "${BRANCH}"
else
  git fetch origin "${BRANCH}:${BRANCH}" || true
  git checkout ${BRANCH}
fi
set -e


set -e	

# change VERSION file	
echo "${VERSION}" > VERSION	

git pull origin "${BRANCH}"	
# git push origin "${BRANCH}"	
	
docker build -t "${QUAY_REPO}" -f ./build/Dockerfile .
#docker push "${QUAY_REPO}"
sed -i "s/IMG=quay.io\/devfile\/devworkspace-controller:.*/IMG=quay.io\/devfile\/devworkspace-controller:${VERSION}/" ./deploy/generate-deployment.sh

./deploy/generate-deployment.sh --use-defaults
# tag the release	
git tag "${VERSION}"	
#   git push origin "${VERSION}"	
COMMIT_MSG="[release] Release ${VERSION}"	
git commit -asm "${COMMIT_MSG}"	

# now update ${BASEBRANCH} to the new snapshot version	
git checkout "${BASEBRANCH}"	

# change VERSION file + commit change into ${BASEBRANCH} branch	
if [[ "${BASEBRANCH}" != "${BRANCH}" ]]; then	
  # bump the y digit, if it is a major release	
  [[ $BRANCH =~ ^([0-9]+)\.([0-9]+)\.x ]] && BASE=${BASH_REMATCH[1]}; NEXT=${BASH_REMATCH[2]}; (( NEXT=NEXT+1 )) # for BRANCH=7.10.x, get BASE=7, NEXT=11	
  NEXT_VERSION_Y="${BASE}.${NEXT}.0-SNAPSHOT"	
  bump_version "${NEXT_VERSION_Y}" "${BASEBRANCH}"	
fi	
# bump the z digit	
[[ $VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]] && BASE="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"; NEXT="${BASH_REMATCH[3]}"; (( NEXT=NEXT+1 )) # for VERSION=7.7.1, get BASE=7.7, NEXT=2	
NEXT_VERSION_Z="${BASE}.${NEXT}-SNAPSHOT"	
bump_version "${NEXT_VERSION_Z}" "${BRANCH}"