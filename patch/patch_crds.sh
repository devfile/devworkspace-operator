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
# This script is required to make automatically generated CRDs compatible with
# Kubernetes >1.18
#
# Patching is done in-place. Note that the formatting output by yq is slightly
# different than the one output by operator-sdk generate commands: list items
# are indented one level further:
#     list:            list:
#     - item1    -->     - item1
#     - item2            - item2
#

set -e

ROUTINGS_CRD_PATH="deploy/templates/crd/bases/controller.devfile.io_devworkspaceroutings.yaml"

# CRD path from root to status field, in jq filter format
STATUS_PATH='.spec.versions[].schema.openAPIV3Schema.properties["status"]'
# CRD path podAdditions to containerPorts required field, in jq filter format
# Note this path takes one jq arg: "CONTAINERS_FIELD" can be used to specify
# "containers" or "initContainers", as both have containerPorts that must be patched.
PODADDITIONS_PATH='.properties["podAdditions"].properties[$CONTAINERS_FIELD].items.properties["ports"].items.required'

# Full jq path to container ports required field in devworkspaceroutings CRD.
ROUTINGS_SPEC_PATH="${STATUS_PATH}${PODADDITIONS_PATH}"

# Update devworkspaceroutings CRD using yq; no-op if already patched.
# Args:
#  $1 - podAdditions field to update ("containers" or "initContainers")
function update_routings_crd() {
  field="$1"
  local yq_check_script="${ROUTINGS_SPEC_PATH}"' | index("protocol")'
  local yq_patch_script="${ROUTINGS_SPEC_PATH}"' |= . + ["protocol"]'
  already_patched=$(yq -r "$yq_check_script" "$ROUTINGS_CRD_PATH" --arg "CONTAINERS_FIELD" "$field")
  if [[ "$already_patched" == "null" ]]; then
    yq -Y -i "$yq_patch_script" "$ROUTINGS_CRD_PATH" --arg "CONTAINERS_FIELD" "$field"
    echo "Patched $1 in CRD ${ROUTINGS_CRD_PATH}"
  else
    echo "Patching $1 in CRD ${ROUTINGS_CRD_PATH} not necessary"
  fi
}

if ! command -v yq 2> /dev/null; then
  echo "Error patching crds: yq is required"
  exit 1
fi

update_routings_crd "containers"
update_routings_crd "initContainers"
