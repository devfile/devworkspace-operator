#!/bin/bash
#
# Copyright (c) 2012-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#

set -e
set -x

# ENV used by PROW ci
export CI="openshift"
export ARTIFACTS_DIR=${ARTIFACT_DIR:-"/tmp/artifacts-che"}
export OPERATOR_REPO=$(dirname $(dirname $(readlink -f "$0")));
export DEVWORKSPACE_CONTROLLER_NAMESPACE="devworkspace-controller"

collectCheLogWithChectl() {
  mkdir -p ${ARTIFACTS_DIR}
  chectl server:logs --chenamespace=${NAMESPACE} --directory=${ARTIFACTS_DIR} --telemetry=off
}

getDevWorkspaceOperatorLogs() {
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
}

# Create admin user inside of openshift cluster and login
function provisionOpenShiftOAuthUser() {
  oc create secret generic htpass-secret --from-file=htpasswd="${OPERATOR_REPO}"/.github/resources/users.htpasswd -n openshift-config
  oc apply -f "${OPERATOR_REPO}"/.github/resources/htpasswdProvider.yaml
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

installOcClient() {
  wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.7.0-rc.1/openshift-client-linux.tar.gz --no-check-certificate -O - | tar -xz
  mv oc /tmp
  PATH=$PATH:/tmp
}

installChectl() {
  wget $(curl https://che-incubator.github.io/chectl/download-link/next-linux-x64)
  tar -xzf chectl-linux-x64.tar.gz
  mv chectl /tmp
  /tmp/chectl/bin/chectl --version
}
