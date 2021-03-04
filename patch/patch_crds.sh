#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
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

COMPONENTS_CRD_PATH="deploy/templates/crd/bases/controller.devfile.io_components.yaml"
ROUTINGS_CRD_PATH="deploy/templates/crd/bases/controller.devfile.io_devworkspaceroutings.yaml"

# CRD path from root to status field, in jq filter format
STATUS_PATH='.spec.versions[].schema.openAPIV3Schema.properties["status"]'
# CRD path podAdditions to containerPorts required field, in jq filter format
# Note this path takes one jq arg: "CONTAINERS_FIELD" can be used to specify
# "containers" or "initContainers", as both have containerPorts that must be patched.
PODADDITIONS_PATH='.properties["podAdditions"].properties[$CONTAINERS_FIELD].items.properties["ports"].items.required'

# Full jq path to container ports required field in components CRD.
COMPONENTS_SPEC_PATH="${STATUS_PATH}"'.properties["componentDescriptions"].items'"${PODADDITIONS_PATH}"
# Full jq path to container ports required field in devworkspaceroutings CRD.
ROUTINGS_SPEC_PATH="${STATUS_PATH}${PODADDITIONS_PATH}"

# Update components CRD using yq; no-op if already patched.
# Args:
#  $1 - podAdditions field to update ("containers" or "initContainers")
function update_components_crd() {
  field="$1"
  local yq_check_script="${COMPONENTS_SPEC_PATH}"' | index("protocol")'
  local yq_patch_script="${COMPONENTS_SPEC_PATH}"' |= . + ["protocol"]'
  already_patched=$(yq -r "$yq_check_script" "$COMPONENTS_CRD_PATH" --arg "CONTAINERS_FIELD" "$field")
  if [[ "$already_patched" == "null" ]]; then
    yq -Y -i "$yq_patch_script" "$COMPONENTS_CRD_PATH" --arg "CONTAINERS_FIELD" "$field"
    echo "Updated CRD $COMPONENTS_CRD_PATH"
  else
    echo "Updating CRD $COMPONENTS_CRD_PATH not necessary"
  fi
}

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
    echo "Updated CRD ${ROUTINGS_CRD_PATH}"
  else
    echo "Updating CRD ${ROUTINGS_CRD_PATH} not necessary"
  fi
}

if ! command -v yq 2> /dev/null; then
  echo "Error patching crds: yq is required"
  exit 1
fi

update_components_crd "containers"
update_components_crd "initContainers"
update_routings_crd "containers"
update_routings_crd "initContainers"
