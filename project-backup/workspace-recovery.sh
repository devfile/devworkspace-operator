#!/usr/bin/env bash
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

set -euo pipefail
set -x

# --- Configuration ---
: "${DEVWORKSPACE_BACKUP_REGISTRY:?Missing DEVWORKSPACE_BACKUP_REGISTRY}"
: "${DEVWORKSPACE_NAMESPACE:?Missing DEVWORKSPACE_NAMESPACE}"
: "${DEVWORKSPACE_NAME:?Missing DEVWORKSPACE_NAME}"
: "${BACKUP_SOURCE_PATH:?Missing BACKUP_SOURCE_PATH}"

BACKUP_IMAGE="${DEVWORKSPACE_BACKUP_REGISTRY}/backup-${DEVWORKSPACE_NAMESPACE}-${DEVWORKSPACE_NAME}:latest"

# --- Functions ---
backup() {
  TARBALL_NAME="devworkspace-backup.tar.gz"
  cd /tmp
  echo "Backing up devworkspace '$DEVWORKSPACE_NAME' in namespace '$DEVWORKSPACE_NAMESPACE' to image '$BACKUP_IMAGE'"

  # Create tarball of the backup source path
  tar -czvf "$TARBALL_NAME" -C "$BACKUP_SOURCE_PATH" .

  # Push the tarball to the OCI registry using oras as a custom artifact
  oras_args=(
    push
    "$BACKUP_IMAGE"
    --artifact-type application/vnd.devworkspace.backup.artifact.v1+json
    --annotation devworkspace.name="$DEVWORKSPACE_NAME"
    --annotation devworkspace.namespace="$DEVWORKSPACE_NAMESPACE"
    --disable-path-validation
  )
  if [[ -n "${REGISTRY_AUTH_FILE:-}" ]]; then
    oras_args+=(--registry-config "$REGISTRY_AUTH_FILE")
  fi
  if [[ -n "${ORAS_EXTRA_ARGS:-}" ]]; then
    extra_args=( ${ORAS_EXTRA_ARGS} )
    oras_args+=("${extra_args[@]}")
  fi
  oras_args+=("$TARBALL_NAME")
  oras "${oras_args[@]}"
  rm -f "$TARBALL_NAME"

  echo "Backup completed successfully."
}

restore() {
  local container_name="workspace-restore"

  podman create --name "$container_name" "$BACKUP_IMAGE"
  rm -rf "${BACKUP_SOURCE_PATH:?}"/*
  podman cp "$container_name":/. "$BACKUP_SOURCE_PATH"
  podman rm "$container_name"
}

usage() {
  echo "Usage: $0 [--backup|--restore]"
  exit 1
}
echo

# --- Main ---
if [[ $# -eq 0 ]]; then
  usage
fi

for arg in "$@"; do
  case "$arg" in
    --backup)
      backup
      ;;
    --restore)
      restore
      ;;
    *)
      echo "Unknown option: $arg"
      usage
      ;;
  esac
done
