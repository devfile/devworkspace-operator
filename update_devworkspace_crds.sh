#!/bin/bash
#
# Copyright (c) 2019-2024 Red Hat, Inc.
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

INIT_ONLY=0

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '--api-version') DEVWORKSPACE_API_VERSION=$2; shift 1;;
    '--init') INIT_ONLY=1; shift 0;;
    *) echo "Unknown argument $1 is specified."; exit 1;;
  esac
  shift 1
done

if [ -z "$DEVWORKSPACE_API_VERSION" ]; then
  echo "Argument --api-version is required"
  exit 1
fi

SCRIPT_DIR=$(cd "$(dirname "$0")" || exit; pwd)

if [[ ("$INIT_ONLY" -eq 1) &&
      (-f "$SCRIPT_DIR/deploy/templates/crd/bases/workspace.devfile.io_devworkspaces.yaml") &&
      (-f "$SCRIPT_DIR/deploy/templates/crd/bases/workspace.devfile.io_devworkspacetemplates.yaml") &&
      ("${DEVWORKSPACE_API_VERSION}" == $(cat "$SCRIPT_DIR/deploy/templates/crd/bases/devfile_version" 2>/dev/null))]]; then
  echo "CRDs from devfile/api are already initialized"
  exit 0
fi

TMP_DIR=$(mktemp -d)
echo "Downloading devfile/api CRDs to $TMP_DIR"

cd "$TMP_DIR"
git init
git remote add origin https://github.com/devfile/api.git
git config core.sparsecheckout true
mkdir -p .git/info
echo "crds/*" > .git/info/sparse-checkout
git fetch --quiet -p origin
if git show-ref --verify refs/tags/"${DEVWORKSPACE_API_VERSION}" --quiet; then
	echo "DevWorkspace API is specified from tag ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet tags/"${DEVWORKSPACE_API_VERSION}"
elif [[ -z $(git ls-remote --heads origin "${DEVWORKSPACE_API_VERSION}") ]]; then
	echo "DevWorkspace API is specified from revision ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}"
else
	echo "DevWorkspace API is specified from branch ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}"
fi
cp crds/workspace.devfile.io_devworkspaces.yaml \
   crds/workspace.devfile.io_devworkspacetemplates.yaml \
   "$SCRIPT_DIR/deploy/templates/crd/bases/"

echo "${DEVWORKSPACE_API_VERSION}" > "$SCRIPT_DIR/deploy/templates/crd/bases/devfile_version"

cd "$SCRIPT_DIR"
