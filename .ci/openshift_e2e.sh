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


#!/usr/bin/env bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
# print each command before executing it
set -x

SCRIPT_DIR=$(dirname $(readlink -f "$0"))
source "${SCRIPT_DIR}"/common.sh

trap 'bumpLogs $?' EXIT SIGINT

export BUMP_LOGS="true"
# Catch the finish of the job and write logs in artifacts.
function bumpLogs() {
    # grab devworkspace-controller namespace events after running e2e
    if [ $BUMP_LOGS == "true" ]; then
        bumpPodsInfo $NAMESPACE
        bumpPodsInfo test-terminal-namespace
        oc get devworkspaces -n test-terminal-namespace -o=yaml > $ARTIFACT_DIR/devworkspaces.yaml
        # export logs once, on failure or after tests are finished
        export BUMP_LOGS="false"
    fi
}

# ENV used by PROW ci
export CI="openshift"
# Use ARTIFACT_DIR from https://docs.ci.openshift.org/docs/architecture/step-registry/#available-environment-variables
# or assign default for local testing
export ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/artifacts}"
export NAMESPACE="devworkspace-controller"
# Component is defined in Openshift CI job configuration. See: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v4.yaml#L8
export CI_COMPONENT="devworkspace-operator"
# DEVWORKSPACE_OPERATOR env var exposed by Openshift CI in e2e test pod. More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
# Dependencies environment are defined here: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v5.yaml#L36-L38

export DWO_IMG=${DEVWORKSPACE_OPERATOR}

# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

# For some reason go on PROW force usage vendor folder
# This workaround is here until we don't figure out cause
go env GOPROXY
go mod tidy
go mod vendor

make install
# configure e2e tests not to clean up the resources since it's just for tests instance
# and we might need to grab data from there
export CLEAN_UP_AFTER_SUITE="false"
make test_e2e
bumpLogs
make uninstall
