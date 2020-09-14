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

DEVWORKSPACE_API_VERSION=${1:-v1alpha1}

mkdir -p devworkspace-crds
cd devworkspace-crds
if [ ! -d ./.git ]; then
	git init
	git remote add origin -f https://github.com/devfile/api.git
	git config core.sparsecheckout true
	echo "deploy/crds/*" > .git/info/sparse-checkout
else
	git remote set-url origin https://github.com/devfile/api.git
fi
git fetch --tags -p origin
if git show-ref --verify refs/tags/"${DEVWORKSPACE_API_VERSION}" --quiet; then
	echo 'DevWorkspace API is specified from tag'
	git checkout tags/"${DEVWORKSPACE_API_VERSION}"
elif git rev-parse --verify "${DEVWORKSPACE_API_VERSION}"; then
	echo 'DevWorkspace API is specified from branch'
	git checkout "${DEVWORKSPACE_API_VERSION}" && git reset --hard origin/"${DEVWORKSPACE_API_VERSION}"
else
	echo 'DevWorkspace API is specified from revision'
	git checkout "${DEVWORKSPACE_API_VERSION}"
fi

