#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

FROM registry.access.redhat.com/ubi9/nodejs-18:1
# hadolint ignore=DL3002
USER 0

ENV GOCACHE="/home/user/.cache/go-build"

# hadolint ignore=DL3041
RUN dnf install -y -q --allowerasing --nobest nodejs-devel nodejs-libs \
  # already installed or installed as deps:
  openssl openssl-devel ca-certificates make cmake cpp gcc gcc-c++ zlib zlib-devel brotli brotli-devel python3 nodejs-packaging && \
  dnf update -y && dnf clean all && \
  npm install -g yarn@1.22 npm@9 && \
  echo -n "node version: "; node -v; \
  echo -n "npm  version: "; npm -v; \
  echo -n "yarn version: "; yarn -v

RUN dnf -y install go && \
  mkdir -p $GOCACHE && \
  dnf update -y && dnf clean all && \
  echo -n "go version: "; go version && \
  dnf install --assumeyes -d1 python3-pip && \
  # Need to pin yq version due to version 3.2.0 requiring python 3.6 and above
  pip3 install yq==v3.1.1 && \
  # install kubectl
  curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
  chmod +x ./kubectl && \
  mv ./kubectl /usr/local/bin && \
  bash <(curl -sL https://www.eclipse.org/che/chectl/) --channel=next
