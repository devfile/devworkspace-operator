#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
set -ex

# Setup to find necessary data from cluster setup
export CI="openshift" # ENV used by PROW ci to get the oc client
export ARTIFACTS_DIR="/tmp/artifacts"
export CUSTOM_HOMEDIR=$ARTIFACTS_DIR

# Overridable information
export DEFAULT_INSTALLER_ASSETS_DIR=${DEFAULT_INSTALLER_ASSETS_DIR:-$(pwd)"/.e2e"}
export KUBEADMIN_USER=${KUBEADMIN_USER:-"kubeadmin"}
export KUBEADMIN_PASSWORD_FILE=${KUBEADMIN_PASSWORD_FILE:-"${DEFAULT_INSTALLER_ASSETS_DIR}/auth/kubeadmin-password"}

# Exported to current env
export KUBECONFIG=${KUBECONFIG:-"${DEFAULT_INSTALLER_ASSETS_DIR}/auth/kubeconfig"}

# Check if file with kubeadmin password exist
if [ ! -f $KUBEADMIN_PASSWORD_FILE ]; then
    echo "Could not find kubeadmin password file"
    exit 1
fi

# Get kubeadmin password from file
export KUBEADMIN_PASSWORD=$(cat $KUBEADMIN_PASSWORD_FILE)

set +x
# Login as admin user
oc login -u $KUBEADMIN_USER -p $KUBEADMIN_PASSWORD --insecure-skip-tls-verify
set -x

# Output of e2e binary
export OUT_FILE=bin/workspace-controller-e2e

# Compile e2e binary tests
CGO_ENABLED=0 go test -v -c -o ${OUT_FILE} ./test/e2e/cmd/workspaces_test.go

# Update CRDs
make update_devworkspace_crds

# Launch tests
./bin/workspace-controller-e2e
