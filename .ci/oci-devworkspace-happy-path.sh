#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
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

SCRIPT_DIR=$(dirname $(readlink -f "$0"))
source "${SCRIPT_DIR}"/common.sh

# Catch the finish of the job and write logs in artifacts.
function Catch_Finish() {
    bumpPodsInfo "devworkspace-controller"
    bumpPodsInfo "eclipse-che"
    USERS_CHE_NS="che-user-che"
    bumpPodsInfo $USERS_CHE_NS
    # Fetch DW related CRs but do not fail when CRDs are not installed yet
    oc get devworkspace -n $USERS_CHE_NS -o=yaml > ${ARTIFACT_DIR}/devworkspaces.yaml || true
    oc get devworkspacetemplate -n $USERS_CHE_NS -o=yaml > ${ARTIFACT_DIR}/devworkspace-templates.yaml || true
    oc get devworkspacerouting -n $USERS_CHE_NS -o=yaml > ${ARTIFACT_DIR}/devworkspace-routings.yaml || true
    /tmp/chectl/bin/chectl server:logs --chenamespace=eclipse-che --directory=${ARTIFACT_DIR}/chectl-server-logs --telemetry=off
}
trap 'Catch_Finish $?' EXIT SIGINT

# ENV used by PROW ci
export CI="openshift"
export NAMESPACE="eclipse-che"
export HAPPY_PATH_POD_NAME=happy-path-che
export HAPPY_PATH_TEST_PROJECT='https://github.com/che-samples/java-spring-petclinic/tree/devfilev2'
# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

deployDWO() {
  export DWO_IMG='${DEVWORKSPACE_OPERATOR}'
  make install
}

deployChe() {
  # create fake DWO CSV to prevent Che Operator getting
  # ownerships of DWO resources
  oc new-project eclipse-che || true
  kubectl apply -f ${SCRIPT_DIR}/resources/fake-dwo-csv.yaml

  /tmp/chectl/bin/chectl server:deploy \
    -p openshift \
    --batch \
    --telemetry=off \
    --installer=operator \
    --workspace-engine=dev-workspace
}

startHappyPathTest() {
  # patch happy-path-che.yaml 
  ECLIPSE_CHE_URL=http://$(oc get route -n "${NAMESPACE}" che -o jsonpath='{.status.ingress[0].host}')
  TS_SELENIUM_DEVWORKSPACE_URL="${ECLIPSE_CHE_URL}/#${HAPPY_PATH_TEST_PROJECT}"
  HAPPY_PATH_POD_FILE=${SCRIPT_DIR}/resources/pod-che-happy-path.yaml
  sed -i "s@CHE_URL@${ECLIPSE_CHE_URL}@g" ${HAPPY_PATH_POD_FILE}
  sed -i "s@WORKSPACE_ROUTE@${TS_SELENIUM_DEVWORKSPACE_URL}@g" ${HAPPY_PATH_POD_FILE}
  sed -i "s@CHE-NAMESPACE@${NAMESPACE}@g" ${HAPPY_PATH_POD_FILE}

  echo "[INFO] Applying the following patched Che Happy Path Pod:"
  cat ${HAPPY_PATH_POD_FILE}
  echo "[INFO] --------------------------------------------------"
  oc apply -f ${HAPPY_PATH_POD_FILE}
  # wait for the pod to start
  n=0
  while [ $n -le 120 ]
  do
    PHASE=$(oc get pod -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} \
        --template='{{ .status.phase }}')
    if [[ ${PHASE} == "Running" ]]; then
      echo "[INFO] Happy-path test started succesfully."
      return
    fi

    sleep 5
    n=$(( n+1 ))
  done

  echo "[ERROR] Failed to start happy-path test."
  exit 1
}

runTest() {
  startHappyPathTest

  # wait for the test to finish
  oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test -f

  # just to sleep
  sleep 3

  # download the test results
  mkdir -p /tmp/e2e
  oc rsync -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME}:/tmp/e2e/report/ /tmp/e2e -c download-reports
  oc exec -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c download-reports -- touch /tmp/done
  cp -r /tmp/e2e ${ARTIFACT_DIR}

  EXIT_CODE=$(oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test | grep EXIT_CODE)

  if [[ ${EXIT_CODE} == "+ EXIT_CODE=1" ]]; then
    echo "[ERROR] Happy-path test failed."
    exit 1
  fi
}

installChectl
provisionOpenShiftOAuthUser
deployDWO
deployChe
runTest
