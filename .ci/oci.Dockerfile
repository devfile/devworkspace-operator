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

FROM registry.ci.openshift.org/openshift/release:golang-1.18

SHELL ["/bin/bash", "-c"]

RUN yum install --assumeyes -d1 python3-pip && \
    pip3 install --upgrade setuptools && \
    # Need to pin yq version due to version 3.2.0 requiring python 3.6 and above
    pip3 install yq==v3.1.1 && \
    # install kubectl
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin && \
    bash <(curl -sL https://www.eclipse.org/che/chectl/) --channel=next
