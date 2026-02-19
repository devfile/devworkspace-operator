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

# --- Functions ---

# Setup registry authentication and return auth arguments for oras
# Args: $1 - registry image (e.g., "registry.example.com/namespace/image:tag")
# Returns: Sets ORAS_AUTH_ARGS array with authentication arguments
setup_registry_auth() {
  local image="$1"
  ORAS_AUTH_ARGS=()

  if [[ -n "${REGISTRY_AUTH_FILE:-}" ]]; then
    # If REGISTRY_AUTH_FILE is provided, use it for authentication
    ORAS_AUTH_ARGS+=(--registry-config "$REGISTRY_AUTH_FILE")
  elif [[ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]]; then
    echo "Using mounted service account token for registry authentication"
    TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    REGISTRY_HOST=$(echo "$image" | cut -d'/' -f1)

    # Create temporary auth config for oras
    REGISTRY_AUTH_FILE="/tmp/registry_auth.json"

    # For OpenShift internal registry, use service account token as password with 'serviceaccount' username
    if [[ "$REGISTRY_HOST" == *"openshift"* ]] || [[ "$REGISTRY_HOST" == *"svc.cluster.local"* ]]; then
      # OpenShift internal registry authentication
      # Use the service account CA for TLS verification
      ORAS_LOGIN_ARGS=(
        login
        --password-stdin
        -u serviceaccount
        --registry-config "$REGISTRY_AUTH_FILE"

      )
      if [[ -n "${ORAS_EXTRA_ARGS:-}" ]]; then
        extra_args=( ${ORAS_EXTRA_ARGS} )
        ORAS_LOGIN_ARGS+=("${extra_args[@]}")
      fi

      if [[ -f /var/run/secrets/kubernetes.io/serviceaccount/ca.crt ]]; then
        ORAS_LOGIN_ARGS+=(--ca-file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt)
      else
        ORAS_LOGIN_ARGS+=(--insecure)
      fi

      oras "${ORAS_LOGIN_ARGS[@]}" "$REGISTRY_HOST" <<< "$TOKEN"
    fi

    ORAS_AUTH_ARGS+=(--registry-config "$REGISTRY_AUTH_FILE")
  fi
}

backup() {
  : "${BACKUP_SOURCE_PATH:?Missing BACKUP_SOURCE_PATH}"
  : "${DEVWORKSPACE_BACKUP_REGISTRY:?Missing DEVWORKSPACE_BACKUP_REGISTRY}"
  : "${DEVWORKSPACE_NAMESPACE:?Missing DEVWORKSPACE_NAMESPACE}"
  : "${DEVWORKSPACE_NAME:?Missing DEVWORKSPACE_NAME}"
  # Remove trailing slash from registry path to avoid double slashes in image reference
  BACKUP_IMAGE="${DEVWORKSPACE_BACKUP_REGISTRY%/}/${DEVWORKSPACE_NAMESPACE}/${DEVWORKSPACE_NAME}:latest"
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

  # Setup registry authentication
  setup_registry_auth "$BACKUP_IMAGE"
  oras_args+=("${ORAS_AUTH_ARGS[@]}")
  if [[ -n "${ORAS_EXTRA_ARGS:-}" ]]; then
    extra_args=( ${ORAS_EXTRA_ARGS} )
    oras_args+=("${extra_args[@]}")
  fi
  oras_args+=("$TARBALL_NAME")
  oras "${oras_args[@]}"
  rm -f "$TARBALL_NAME"

  # Clean up temporary auth file if created
  if [[ -f /tmp/registry_auth.json ]]; then
    rm -f /tmp/registry_auth.json
  fi

  echo "Backup completed successfully."
}

restore() {
  : "${BACKUP_IMAGE:?Missing BACKUP_IMAGE}"
  : "${PROJECTS_ROOT:?Missing PROJECTS_ROOT}"

  echo "Restoring devworkspace from image '$BACKUP_IMAGE' to path '$PROJECTS_ROOT'"
  oras_args=(
    pull
    "$BACKUP_IMAGE"
    --output /tmp
  )

  # Setup registry authentication
  setup_registry_auth "$BACKUP_IMAGE"
  oras_args+=("${ORAS_AUTH_ARGS[@]}")

  if [[ -n "${ORAS_EXTRA_ARGS:-}" ]]; then
    extra_args=( ${ORAS_EXTRA_ARGS} )
    oras_args+=("${extra_args[@]}")
  fi

  # Pull the backup tarball from the OCI registry using oras and extract it
  oras "${oras_args[@]}"
  mkdir /tmp/extracted-backup
  tar -xzvf /tmp/devworkspace-backup.tar.gz -C /tmp/extracted-backup

  # Check if $PROJECTS_ROOT is empty and exit if not
  if [[ -n "$(ls -A "$PROJECTS_ROOT")" ]]; then
    echo "PROJECTS_ROOT '$PROJECTS_ROOT' is not empty. Skipping restore action."
    exit 0
  fi

  cp -r /tmp/extracted-backup/* "$PROJECTS_ROOT"

  rm -f /tmp/devworkspace-backup.tar.gz
  rm -rf /tmp/extracted-backup

  echo "Restore completed successfully."
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
