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

: ${1?'missing key directory'}

key_dir="$1"

chmod 0700 "$key_dir"
cd "$key_dir"

# Generate the CA cert and private key
openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -days 1024 -subj "/CN=Admission Workspace Controller Webhook"
# Generate the private key for the webhook server
openssl genrsa -out webhook-server-tls.key 2048
# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
openssl req -new -key webhook-server-tls.key -subj "/CN=workspace-controller.che-workspace-controller.svc" \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -days 365 -out webhook-server-tls.crt
