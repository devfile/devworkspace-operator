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

trap 'Catch_Finish $?' EXIT SIGINT

# Catch the finish of the job and write logs in artifacts.
function Catch_Finish() {
    # grab devworkspace-controller namespace events after running e2e
    getDevWorkspaceOperatorLogs
}

# ENV used by PROW ci
export CI="openshift"
export ARTIFACTS_DIR="/tmp/artifacts"
export NAMESPACE="devworkspace-controller"

# Component is defined in Openshift CI job configuration. See: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v4.yaml#L8
export CI_COMPONENT="devworkspace-operator"

# DEVWORKSPACE_OPERATOR env var exposed by Openshift CI in e2e test pod. More info about how images are builded in Openshift CI: https://github.com/openshift/ci-tools/blob/master/TEMPLATES.md#parameters-available-to-templates
# Dependencies environment are defined here: https://github.com/openshift/release/blob/master/ci-operator/config/devfile/devworkspace-operator/devfile-devworkspace-operator-master__v5.yaml#L36-L38
export IMG=${DEVWORKSPACE_OPERATOR}

# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

# Function to get all logs and events from devworkspace operator deployments
function getDevWorkspaceOperatorLogs() {
    mkdir -p ${ARTIFACTS_DIR}/devworkspace-operator
    cd ${ARTIFACTS_DIR}/devworkspace-operator
    for POD in $(oc get pods -o name -n ${NAMESPACE}); do
       for CONTAINER in $(oc get -n ${NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
            echo ""
            echo "<=========================Getting logs from $POD==================================>"
            echo ""
            oc logs ${POD} -c ${CONTAINER} -n ${NAMESPACE} | tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
        done
    done
    echo "======== oc get events ========"
    oc get events -n ${NAMESPACE}| tee get_events.log
}

# For some reason go on PROW force usage vendor folder
# This workaround is here until we don't figure out cause
go mod tidy
go mod vendor
make test_e2e
