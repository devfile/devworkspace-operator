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

DEVWORKSPACE_API_VERSION=v1alpha1
INIT_ONLY=0

while [[ "$#" -gt 0 ]]; do
  case $1 in
    '--api-version') DEVWORKSPACE_API_VERSION=$2; shift 1;;
    '--init') INIT_ONLY=1; shift 0;;
    *) echo "Unknown argument $1 is specified."; exit 1;;
  esac
  shift 1
done

SCRIPT_DIR=$(cd "$(dirname "$0")" || exit; pwd)

if [[ ($INIT_ONLY -eq 1) && (-f "$SCRIPT_DIR/config/crd/bases/workspace.devfile.io_devworkspaces.yaml") ]]; then
  # devworkspace crd is already initialized
  exit 0
fi

TMP_DIR=$(mktemp -d)
echo "Downloading devfile/api CRDs to $TMP_DIR"

cd $TMP_DIR
git init
git remote add origin https://github.com/devfile/api.git
git config core.sparsecheckout true
echo "deploy/crds/*" > .git/info/sparse-checkout
git fetch --quiet --tags -p origin 
if git show-ref --verify refs/tags/"${DEVWORKSPACE_API_VERSION}" --quiet; then
	echo 'DevWorkspace API is specified from tag'
	git checkout --quiet tags/"${DEVWORKSPACE_API_VERSION}"
elif git rev-parse --verify "${DEVWORKSPACE_API_VERSION}"; then
	echo 'DevWorkspace API is specified from branch'
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}" && git reset --hard origin/"${DEVWORKSPACE_API_VERSION}"
else
	echo 'DevWorkspace API is specified from revision'
	git checkout --quiet "${DEVWORKSPACE_API_VERSION}"
fi

cp deploy/crds/workspace.devfile.io_devworkspaces_crd.yaml \
   $SCRIPT_DIR/config/crd/bases/workspace.devfile.io_devworkspaces.yaml

cd $SCRIPT_DIR
