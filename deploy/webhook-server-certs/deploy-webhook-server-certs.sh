#
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

# Generate a (self-signed) CA certificate and a certificate and private key to be used by the webhook server.
# The certificate will be issued for the Common Name (CN) of `workspace-controller.che-workspace-controller.svc`,
# which is the cluster-internal DNS name for the service.
#
# NOTE: THIS SCRIPT EXISTS FOR TEST PURPOSES ONLY. DO NOT USE IT FOR YOUR PRODUCTION WORKLOADS.

set -e

CLI=$1

echo "Generating new TLS certificates using docker"
PROJECT_FOLDER=$(dirname "$0")/../..
docker build --no-cache -t generate-webhook-server-certs:latest ${PROJECT_FOLDER}/deploy/webhook-server-certs

TARGET_FOLDER=$PROJECT_FOLDER/build/_output/webhook-certs
mkdir -p $TARGET_FOLDER

echo "Copying generated TLS certificates from docker container"
docker run --name 'webhook-certs' generate-webhook-server-certs:latest exit 0
docker cp webhook-certs:ca/. ${TARGET_FOLDER}/
docker rm 'webhook-certs'

$CLI delete secret -n che-workspace-controller webhook-server-tls --ignore-not-found=true
$CLI -n che-workspace-controller create secret tls webhook-server-tls \
    --cert "$TARGET_FOLDER/webhook-server-tls.crt" \
    --key "$TARGET_FOLDER/webhook-server-tls.key"
CA_BASE_64_CONTENT="$(openssl base64 -A <"${TARGET_FOLDER}/ca.crt")"
$CLI patch -n che-workspace-controller secret webhook-server-tls -p="{\"data\":{\"ca.crt\": \"${CA_BASE_64_CONTENT}\"}}"
echo "TLS certificates are stored in 'che-workspace-controller' namespace in 'webhook-server-tls' secret"

rm -r ${TARGET_FOLDER}
