#
# Copyright (c) 2019-2025 Red Hat, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

FROM registry.ci.openshift.org/openshift/release:rhel-9-release-golang-1.24-openshift-4.20
FROM registry.ci.openshift.org/openshift/release:rhel-9-release-golang-1.24-openshift-4.20

ENV GO_VERSION=1.25.7
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH

ENV GO_VERSION=1.25.7
ENV GOROOT=/usr/local/go
ENV PATH=$GOROOT/bin:$PATH

SHELL ["/bin/bash", "-c"]

# Install Go 1.25.7 to satisfy go.mod toolchain requirement (go 1.25.0)
RUN export ARCH="$(uname -m)" && if [[ ${ARCH} == "x86_64" ]]; then export ARCH="amd64"; elif [[ ${ARCH} == "aarch64" ]]; then export ARCH="arm64"; fi && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o go.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz
RUN go version

# Temporary workaround since mirror.centos.org is down and can be replaced with vault.centos.org
RUN sed -i s/mirror.centos.org/vault.centos.org/g /etc/yum.repos.d/*.repo && sed -i s/^#.*baseurl=http/baseurl=http/g /etc/yum.repos.d/*.repo && sed -i s/^mirrorlist=http/#mirrorlist=http/g /etc/yum.repos.d/*.repo

RUN yum install --assumeyes -d1 python3-pip nodejs gettext jq && \
    pip3 install --upgrade pip && \
    pip3 install --ignore-installed --upgrade setuptools && \
    # Install yq and jq
    pip3 install yq jq && \
    # Install kubectl
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin && \
    # Install chectl
    bash <(curl -sL https://che-incubator.github.io/chectl/install.sh) --channel=next
