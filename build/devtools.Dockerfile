#
# Copyright (c) 2019-2024 Red Hat, Inc.
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
FROM quay.io/devfile/universal-developer-image:latest

USER root

# Install gettext
RUN dnf install -y gettext

# Install the Operator SDK
ENV OPERATOR_SDK_VERSION="v1.8.0"
ENV OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}
RUN curl -sSLO ${OPERATOR_SDK_DL_URL}/operator-sdk_linux_amd64 && \
    gpg --keyserver keyserver.ubuntu.com --recv-keys 052996E2A20B5C7E && \
    curl -sSLO ${OPERATOR_SDK_DL_URL}/checksums.txt && \
    curl -sSLO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc && \
    gpg -u "Operator SDK (release) <cncf-operator-sdk@cncf.io>" --verify checksums.txt.asc && \
    grep operator-sdk_linux_amd64 checksums.txt | sha256sum -c - && \
    chmod +x operator-sdk_linux_amd64 && \
    mv operator-sdk_linux_amd64 /usr/local/bin/operator-sdk && \
    rm checksums.txt checksums.txt.asc

# Install opm CLI
ENV OPM_VERSION="v1.19.5"
RUN curl -sSLO https://github.com/operator-framework/operator-registry/releases/download/${OPM_VERSION}/linux-amd64-opm && \
    chmod +x linux-amd64-opm && \
    mv linux-amd64-opm /usr/local/bin/opm

USER 1001
