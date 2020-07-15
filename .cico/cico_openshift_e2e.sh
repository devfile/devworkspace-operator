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

# ENV used by PROW ci
export CI="openshift" 
export ARTIFACTS_DIR="/tmp/artifacts"
export DOCKER_IMAGE=registry.svc.ci.openshift.org/${OPENSHIFT_BUILD_NAMESPACE}/stable:devworkspace-operator
export SCRIPT=$(readlink -f "$0")
export SCRIPTPATH=$(dirname "$SCRIPT") 
export OPERATOR_REPO=$(dirname "$SCRIPTPATH")

sed -i.bak -e "s|image: .*|image: ${DOCKER_IMAGE}|g" ${OPERATOR_REPO}/deploy/os/controller.yaml

# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"
####TEEEST
# Check if operator-sdk is installed and if not install operator sdk in $GOPATH/bin dir
if ! hash operator-sdk 2>/dev/null; then
    mkdir -p $GOPATH/bin
    export PATH="$PATH:$(pwd):$GOPATH/bin"
    OPERATOR_SDK_VERSION=v0.17.0

    curl -LO https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}/operator-sdk-${OPERATOR_SDK_VERSION}-x86_64-linux-gnu

    chmod +x operator-sdk-${OPERATOR_SDK_VERSION}-x86_64-linux-gnu && \
        cp operator-sdk-${OPERATOR_SDK_VERSION}-x86_64-linux-gnu $GOPATH/bin/operator-sdk && \
        rm operator-sdk-${OPERATOR_SDK_VERSION}-x86_64-linux-gnu
fi

# Add kubernetes-api CRDS
make update_devworkspace_crds

# Install go modules
go mod tidy
go mod vendor

# Output of e2e binary
export OUT_FILE=bin/workspace-controller-e2e

# Compile e2e binary tests
CGO_ENABLED=0 go test -v -c -o ${OUT_FILE} ./test/e2e/cmd/workspaces_test.go

# Launch tests
./bin/workspace-controller-e2e
