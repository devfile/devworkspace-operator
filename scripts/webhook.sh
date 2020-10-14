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

NAMESPACE=""
printHelp="false"
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-n'|'--namespace') NAMESPACE="$2"; shift 1;;
    '-h'|'--help') printHelp="true"; shift 0;;
  esac
  shift 1
done

if [[ "$printHelp" == "true" ]]; then
  echo "Usage:"
  echo "-n, --namespace - The namespace where the webhooks will live"
  echo "-h, --help - print this message"
  exit 0
fi

if [[ "$NAMESPACE" == "" ]]; then
  echo "Namespace cannot be empty"
  exit 1
fi

IS_NOT_FOUND=$(kubectl get namespace cert-manager 2>&1 | grep -q "not found"; echo $?)
if [ "$IS_NOT_FOUND" = "0" ]; then
  echo "Cert Manager has not been found.";
else
  echo "$NAMESPACE"
  sed -i.bak -e "s|namespace: .*|namespace: $NAMESPACE|" \
           -e "s|devworkspace-webhookserver.devworkspace-controller.svc|devworkspace-webhookserver.$NAMESPACE.svc|" \
            deploy/k8s_webhook/cert_manager.yaml
  kubectl apply -f deploy/k8s_webhook/cert_manager.yaml -n "$NAMESPACE"
  mv deploy/k8s_webhook/cert_manager.yaml.bak deploy/k8s_webhook/cert_manager.yaml
fi
