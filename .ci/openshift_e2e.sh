#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
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

function bumpPodsInfo() {
    NS=$1
    TARGET_DIR="${ARTIFACT_DIR}/${NS}-info"
    mkdir -p $TARGET_DIR

    for POD in $(oc get pods -o name -n ${NS}); do
        for CONTAINER in $(oc get -n ${NS} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
            echo ""
            echo "======== Getting logs from container $POD/$CONTAINER in $NS"
            echo ""
            # container name includes `pod/` prefix. remove it
            LOGS_FILE=$TARGET_DIR/$(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
            oc logs ${POD} -c ${CONTAINER} -n ${NS} > $LOGS_FILE || true
        done
    done
    echo "======== Bumping events -n ${NS} ========"
    oc get events -n ${NS} -o=yaml > $TARGET_DIR/events.log || true
}

trap 'bumpLogs $?' EXIT SIGINT

export BUMP_LOGS="true"
# Catch the finish of the job and write logs in artifacts.
function bumpLogs() {
    # grab devworkspace-controller namespace events after running e2e
    if [ $BUMP_LOGS == "true" ]; then
        bumpPodsInfo $NAMESPACE
        bumpPodsInfo test-terminal-namespace
        oc get devworkspaces -n test-terminal-namespace -o=yaml > $ARTIFACTS_DIR/devworkspaces.yaml
        # export logs once, on failure or after tests are finished
        export BUMP_LOGS="false"
    fi
}

# ENV used by PROW ci
export CI="openshift"
export ARTIFACTS_DIR="/tmp/artifacts"
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
go mod tidy
go mod vendor

make install
# configure e2e tests not to clean up the resources since it's just for tests instance
# and we might need to grab data from there
export CLEAN_UP_AFTER_SUITE="false"
make test_e2e
bumpLogs
make uninstall
