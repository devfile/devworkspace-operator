#!/usr/bin/env bash
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
  local new_image
  new_image=$(buildah from scratch)

  echo "Backing up workspace from path: $BACKUP_SOURCE_PATH"
  ls -la "$BACKUP_SOURCE_PATH"

  buildah copy "$new_image" "$BACKUP_SOURCE_PATH" /
  buildah config --label DEVWORKSPACE="$DEVWORKSPACE_NAME" "$new_image"
  buildah config --label NAMESPACE="$DEVWORKSPACE_NAMESPACE" "$new_image"
  buildah commit "$new_image" "$BACKUP_IMAGE"

  buildah umount "$new_image"
  buildah push ${BUILDAH_PUSH_OPTIONS:-} "$BACKUP_IMAGE"
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
