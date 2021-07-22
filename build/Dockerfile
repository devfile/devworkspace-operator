# Copyright (c) 2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8/go-toolset
FROM registry.access.redhat.com/ubi8/go-toolset:1.15.7-11 as builder
ENV GOPATH=/go/
USER root
WORKDIR /devworkspace-operator
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go env GOPROXY && \
    go mod download

# Copy the go source
COPY . .

# compile workspace controller binaries, then webhook binaries
RUN make compile-devworkspace-controller
RUN make compile-webhook-server

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.4-205.1626828526
RUN microdnf -y update && microdnf clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
WORKDIR /
COPY --from=builder /devworkspace-operator/_output/bin/devworkspace-controller /usr/local/bin/devworkspace-controller
COPY --from=builder /devworkspace-operator/_output/bin/webhook-server /usr/local/bin/webhook-server
COPY --from=builder /devworkspace-operator/internal-registry internal-registry

ENV USER_UID=1001 \
    USER_NAME=devworkspace-controller

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/devworkspace-controller

# append Brew metadata here
