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

# Component is defined in Openshift CI job configuration. See: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v4.yaml#L8
export CI_COMPONENT="devworkspace-operator"
# OPENSHIFT_BUILD_NAMESPACE env var exposed by Openshift CI. More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
export CI_CONTROLLER_IMAGE=registry.svc.ci.openshift.org/${OPENSHIFT_BUILD_NAMESPACE}/stable:${CI_COMPONENT}

# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

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

# For some reason go on PROW force usage vendor folder
# This workaround is here until we don't figure out cause
go mod tidy
go mod vendor

make test_e2e

# grab devworkspace-controller namespace events after running e2e
oc get events -n devworkspace-controller | tee ${ARTIFACTS_DIR}/devworkspace-controller-events.log
