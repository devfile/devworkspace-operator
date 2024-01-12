#!/bin/bash
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


#!/usr/bin/env bash
# exit immediately when a command fails
set -e
# only exit with zero if all commands of the pipeline exit successfully
set -o pipefail
# error on unset variables
set -u
# print each command before executing it
set -x

# ENV used by PROW ci
export CI="openshift"
# Pod created by openshift ci don't have user. Using this envs should avoid errors with git user.
export GIT_COMMITTER_NAME="CI BOT"
export GIT_COMMITTER_EMAIL="ci_bot@notused.com"

deployDWO() {
  export NAMESPACE="devworkspace-controller"
  export DWO_IMG="${DEVWORKSPACE_OPERATOR}"
  make install
}

deployChe() {
  chectl server:deploy \
    -p openshift \
    --batch \
    --telemetry=off \
    --installer=operator \
    --workspace-engine=dev-workspace
}

deployDWO
deployChe
export CHE_REPO_BRANCH="main"
bash <(curl -s https://raw.githubusercontent.com/eclipse/che/${CHE_REPO_BRANCH}/tests/devworkspace-happy-path/remote-launch.sh)
