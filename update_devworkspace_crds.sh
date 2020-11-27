#!/bin/bash
# Copyright (c) 2019-2020 Red Hat, Inc.
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

INIT_ONLY=0

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '--api-version') DEVWORKSPACE_API_VERSION=$2; shift 1;;
    '--init') INIT_ONLY=1; shift 0;;
    *) echo "Unknown argument $1 is specified."; exit 1;;
  esac
  shift 1
done

if [ -z $DEVWORKSPACE_API_VERSION ]; then
  echo "Argument --api-version is required"
  exit 1
fi

SCRIPT_DIR=$(cd "$(dirname "$0")" || exit; pwd)

if [[ ($INIT_ONLY -eq 1) &&
      (-f "$SCRIPT_DIR/config/crd/bases/workspace.devfile.io_devworkspaces.yaml") &&
      (-f "$SCRIPT_DIR/config/crd/bases/workspace.devfile.io_devworkspacetemplates.yaml")]]; then
  echo "CRDs from devfile/api are already initialized"
  exit 0
fi

TMP_DIR=$(mktemp -d)
echo "Downloading devfile/api CRDs to $TMP_DIR"

cd $TMP_DIR
git init
git remote add origin https://github.com/devfile/api.git
git config core.sparsecheckout true
echo "crds/*" > .git/info/sparse-checkout
git fetch --quiet -p origin
if git show-ref --verify refs/tags/"${DEVWORKSPACE_API_VERSION}" --quiet; then
	echo "DevWorkspace API is specified from tag ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet tags/"${DEVWORKSPACE_API_VERSION}"
elif [[ -z $(git ls-remote --heads origin ${DEVWORKSPACE_API_VERSION}) ]]; then
	echo "DevWorkspace API is specified from revision ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}"
else
	echo "DevWorkspace API is specified from branch ${DEVWORKSPACE_API_VERSION}"
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}"
fi
cp crds/workspace.devfile.io_devworkspaces.yaml \
   crds/workspace.devfile.io_devworkspacetemplates.yaml \
   $SCRIPT_DIR/config/crd/bases/

cd $SCRIPT_DIR
