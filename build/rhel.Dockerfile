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

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.13.15-4  as builder
ENV PATH=/opt/rh/go-toolset-1.13/root/usr/bin:${PATH} \
    GOPATH=/go/
USER root

RUN go env GOPROXY
WORKDIR /devworkspace-operator
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# NOTE: downstream this needs to be a tarball, not a live download
RUN go mod tidy; go mod vendor

# Copy the go source
COPY . .

# compile workspace controller binaries, then webhook binaries
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
  CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} GO111MODULE=on go build -mod=vendor -x \
  -a -o _output/bin/devworkspace-controller \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  main.go
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
  CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} GO111MODULE=on go build -mod=vendor -x \
  -o _output/bin/webhook-server \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  webhook/main.go

# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.3-298
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
