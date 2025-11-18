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

  cat <<EOF > /home/podman/Dockerfile.backup
FROM scratch
COPY "$BACKUP_SOURCE_PATH" /
LABEL DEVWORKSPACE="$DEVWORKSPACE_NAME"
LABEL NAMESPACE="$DEVWORKSPACE_NAMESPACE"
EOF
  podman build \
  --file /home/podman/Dockerfile.backup \
  --tag "$BACKUP_IMAGE" /

  podman push ${PODMAN_PUSH_OPTIONS:-} "$BACKUP_IMAGE"

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
