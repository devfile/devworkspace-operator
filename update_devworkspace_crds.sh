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

set -ex

DEVWORKSPACE_API_VERSION=${1:-v1alpha1}

SCRIPT_DIR=$(cd "$(dirname "$0")" || exit; pwd)
TMP_DIR=$(mktemp -d)
echo "Downloading devfile/api CRDs to $TMP_DIR"

cd $TMP_DIR
# mkdir -p devworkspace-crds
# cd devworkspace-crds
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