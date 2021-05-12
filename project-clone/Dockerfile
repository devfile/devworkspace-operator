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

# Build the manager binary
# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/devtools/go-toolset-rhel7
FROM registry.access.redhat.com/devtools/go-toolset-rhel7:1.13.15-4  as builder
ENV PATH=/opt/rh/go-toolset-1.13/root/usr/bin:${PATH} \
    GOPATH=/go/
USER root

WORKDIR /project-clone
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# compile workspace controller binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
  -a -o _output/bin/project-clone \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  project-clone/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# https://access.redhat.com/containers/?tab=tags#/registry.access.redhat.com/ubi8-minimal
FROM registry.access.redhat.com/ubi8-minimal:8.3-298.1618432845
RUN microdnf -y update && microdnf clean all && rm -rf /var/cache/yum && echo "Installed Packages" && rpm -qa | sort -V && echo "End Of Installed Packages"
WORKDIR /
COPY --from=builder /project-clone/_output/bin/project-clone /usr/local/bin/project-clone

ENV USER_UID=1001 \
    USER_NAME=project-clone

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/project-clone

# append Brew metadata here
