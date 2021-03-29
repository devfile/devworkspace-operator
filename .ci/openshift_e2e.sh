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

trap 'Catch_Finish $?' EXIT SIGINT

# Catch the finish of the job and write logs in artifacts.
function Catch_Finish() {
    # grab devworkspace-controller namespace events after running e2e
    getDevWorkspaceOperatorLogs
}

# ENV used by PROW ci
export CI="openshift"
export ARTIFACTS_DIR="/tmp/artifacts"
export NAMESPACE="eclipse-che"
export DEVWORKSPACE_CONTROLLER_NAMESPACE="devworkspace-controller"
export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
export HAPPY_PATH_POD_NAME=happy-path-che
export HAPPY_PATH_DEVFILE='https://gist.githubusercontent.com/l0rd/71a04dd0d8c8e921b16ba2690f7d5a47/raw/d520086e148c359b18c229328824dfefcf85e5ef/spring-petclinic-devfile-v2.0.0.yaml'

# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

function getDevWorkspaceOperatorLogs() {
    mkdir -p ${ARTIFACTS_DIR}/devworkspace-operator
    cd ${ARTIFACTS_DIR}/devworkspace-operator
    for POD in $(oc get pods -o name -n ${DEVWORKSPACE_CONTROLLER_NAMESPACE}); do
       for CONTAINER in $(oc get -n ${DEVWORKSPACE_CONTROLLER_NAMESPACE} ${POD} -o jsonpath="{.spec.containers[*].name}"); do
            echo ""
            echo "<=========================Getting logs from container $CONTAINER in $POD==================================>"
            echo ""
            oc logs ${POD} -c ${CONTAINER} -n ${DEVWORKSPACE_CONTROLLER_NAMESPACE} | tee $(echo ${POD}-${CONTAINER}.log | sed 's|pod/||g')
        done
    done
    echo "======== oc get events ========"
    oc get events -n ${DEVWORKSPACE_CONTROLLER_NAMESPACE}| tee get_events.log

  mkdir -p ${ARTIFACTS_DIR}
  /tmp/chectl/bin/chectl server:logs --chenamespace=${NAMESPACE} --directory=${ARTIFACTS_DIR}
}

deployChe() {
  cat > /tmp/che-cr-patch.yaml <<EOL
spec:
  devWorkspace:
    enable: true
  server:
    customCheProperties:
      CHE_FACTORY_DEFAULT__PLUGINS: ""
      CHE_WORKSPACE_DEVFILE_DEFAULT__EDITOR_PLUGINS: ""
  auth:
    updateAdminPassword: false
EOL

  cat /tmp/che-cr-patch.yaml

  /tmp/chectl/bin/chectl server:deploy --che-operator-cr-patch-yaml=/tmp/che-cr-patch.yaml -p openshift --batch --telemetry=off --installer=operator --dev-workspace-controller-image=${DEVWORKSPACE_OPERATOR}
}

# Create admin user inside of openshift cluster and login
function provisionOpenShiftOAuthUser() {
  oc create secret generic htpass-secret --from-file=htpasswd="${OPERATOR_REPO}"/.github/users.htpasswd -n openshift-config
  oc apply -f "${OPERATOR_REPO}"/.github/htpasswdProvider.yaml
  oc adm policy add-cluster-role-to-user cluster-admin user

  echo -e "[INFO] Waiting for htpasswd auth to be working up to 5 minutes"
  CURRENT_TIME=$(date +%s)
  ENDTIME=$(($CURRENT_TIME + 300))
  while [ $(date +%s) -lt $ENDTIME ]; do
      if oc login -u user -p user --insecure-skip-tls-verify=false; then
          break
      fi
      sleep 10
  done
}

startHappyPathTest() {
  # patch happy-path-che.yaml 
  ECLIPSE_CHE_URL=http://$(oc get route -n "${NAMESPACE}" che -o jsonpath='{.status.ingress[0].host}')
  TS_SELENIUM_DEVWORKSPACE_URL="${ECLIPSE_CHE_URL}/#${HAPPY_PATH_DEVFILE}"
  sed -i "s@CHE_URL@${ECLIPSE_CHE_URL}@g" ${OPERATOR_REPO}/.ci/happy-path-che.yaml
  sed -i "s@WORKSPACE_ROUTE@${TS_SELENIUM_DEVWORKSPACE_URL}@g" ${OPERATOR_REPO}/.ci/happy-path-che.yaml
  sed -i "s@CHE-NAMESPACE@${NAMESPACE}@g" ${OPERATOR_REPO}/.ci/happy-path-che.yaml
  cat ${OPERATOR_REPO}/.ci/happy-path-che.yaml

  oc apply -f ${OPERATOR_REPO}/.ci/happy-path-che.yaml
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

installChectl() {
  curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
  chmod +x ./kubectl 
  mv ./kubectl /tmp 

  wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.7.0-rc.1/openshift-client-linux.tar.gz --no-check-certificate -O - | tar -xz
  mv oc /tmp

  # curl -sL https://www.eclipse.org/che/chectl/ > install_chectl.sh
  # chmod +x install_chectl.sh
  # ./install_chectl.sh --channel=next

  wget https://github.com/che-incubator/chectl/releases/download/20210324120946/chectl-linux-x64.tar.gz
  tar -xzf chectl-linux-x64.tar.gz
  mv chectl /tmp
  /tmp/chectl/bin/chectl --version
}

runTest() {
  deployChe

  startHappyPathTest

  # wait for the test to finish
  oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test -f

  # just to sleep
  sleep 3

  # download the test results
  mkdir -p /tmp/e2e
  oc rsync -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME}:/tmp/e2e/report/ /tmp/e2e -c download-reports
  oc exec -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c download-reports -- touch /tmp/done

  mkdir -p ${ARTIFACTS_DIR}
  cp -r /tmp/e2e ${ARTIFACTS_DIR}

  EXIT_CODE=$(oc logs -n ${NAMESPACE} ${HAPPY_PATH_POD_NAME} -c happy-path-test | grep EXIT_CODE)

  if [[ ${EXIT_CODE} == "+ EXIT_CODE=1" ]]; then
    echo "[ERROR] Happy-path test failed."
    exit 1
  fi
}

installChectl
provisionOpenShiftOAuthUser
runTest
