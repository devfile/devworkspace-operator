#!/bin/bash

set -eo pipefail

PODMAN=podman
MULTI_ARCH="false"
ARCHITECTURES="linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"
SCRIPT_DIR=$(cd "$(dirname "$0")" || exit; pwd)

DEFAULT_BUILDER_NAME="dwo-multi-platform-builder"

usage() {
  cat <<EOF
This script converts a operator bundle image that may have images referred to by
tags into the equivalent bundle that references images only through their image
digests.

Requires skopeo, podman, opm v1.19.5, and kislyuk/yq.

Arguments:
  --bundle <IMAGE>        : Bundle image to process (required).
  --render <PATH>         : Render processed bundle image to <PATH> (required). Filename of rendered bundle will be
                            determined from bundle name.
  --push <IMAGE>          : Push processed digests bundle to <IMAGE> (required). Pushing the bundle image to a
                            repository is required because opm render works from a remote repository.
  --container-tool <TOOL> : Use specific container tool for building/pushing images (default: use podman).
  --multi-arch            : Create images for the following architectures: $ARCHITECTURES. Note: Docker buildx will be
                            used for building and pushing the images, instead of the container tool selected with --container-tool.
  --debug, -d             : Print debug information.
  --help, -h              : Show this message.
EOF
}

error() {
  echo "[ERROR] $1"
  exit 1
}

info() {
  echo "[INFO]  $1"
}

debug() {
  if [ "$DEBUG" == "true" ]; then
    echo "[DEBUG] $1"
  fi
}

parse_args() {
  if [ "$#" -eq 0 ]; then
    usage
    exit 0
  fi
  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--bundle') BUNDLE="$2"; shift;;
      '--render') RENDER="$2"; shift;;
      '--push') PUSH_IMAGE="$2"; shift;;
      '--container-tool') PODMAN="$2"; shift;;
      '--multi-arch') MULTI_ARCH="true";;
      '-d'|'--debug') DEBUG="true";;
      '-h'|'--help') usage; exit 0;;
      *) error "Unknown parameter is used: $1."; usage; exit 1;;
    esac
    shift 1
  done
}

preflight() {
  if [ -z "$BUNDLE" ]; then
    error "Argument --bundle is required"
  fi
  if [ -z "$RENDER" ]; then
    error "Argument --render is required"
  fi
  if [ -z "$PUSH_IMAGE" ]; then
    error "Argument --push is required"
  fi
  if [ ! -d "$RENDER" ]; then
    error "Directory $RENDER does not exist"
  fi
  # Reuse usual bundle.Dockerfile from repo
  if [ ! -f "${SCRIPT_DIR}/../bundle.Dockerfile" ]; then
    error "Could not find bundle.Dockerfile from repository"
  fi
}

parse_args "$@"
preflight

if [ "$MULTI_ARCH" == "true" ]; then
  # Create a multi-arch builder if one doesn't already exist
  BUILDER_EXISTS=0
  docker buildx use "$DEFAULT_BUILDER_NAME" || BUILDER_EXISTS=$?

  if [ $BUILDER_EXISTS -eq 0 ]; then
    echo "Using $DEFAULT_BUILDER_NAME for build"
  else
    echo "Setting up Docker buildx builder:"
    docker buildx create --name "$DEFAULT_BUILDER_NAME" --driver docker-container --config build/buildkitd.toml --use
  fi
fi

# Work in a temporary directory
TMPDIR="$(mktemp -d)"
info "Working in $TMPDIR"
BUNDLE_DIR="${TMPDIR}/bundle"
mkdir -p "$BUNDLE_DIR"
PROCESSED_DIR="${TMPDIR}/bundle-processed"
mkdir -p "$PROCESSED_DIR"

# Create a container so that we can export its filesystem
CONTAINER=$($PODMAN create --name temp_bundle "$BUNDLE" --entrypoint /bin/sh)
# Export filesystem to ./bundle.tar
$PODMAN export "$CONTAINER" -o "$TMPDIR/bundle.tar"
# Remove container; no longer needed
$PODMAN rm "$CONTAINER" >/dev/null
# Extract manifests, metadata, and tests dirs
tar -xf "$TMPDIR/bundle.tar" --directory "$BUNDLE_DIR"

# Make a copy of the input bundle for modifying, so that we can show a diff later
cp -r "$BUNDLE_DIR"/* "$PROCESSED_DIR"

for FILE in "$PROCESSED_DIR"/manifests/*.clusterserviceversion.yaml; do
  info "Processing file $FILE"
  IMAGES=$(yq -r '.spec.relatedImages[].image' "$FILE")
  for IMAGE in $IMAGES; do
    DIGEST=$(skopeo inspect --tls-verify=false "docker://${IMAGE}" 2>/dev/null | jq -r '.Digest')
    IMAGE_WITH_DIGEST="${IMAGE%%:*}@${DIGEST}"
    info "Replacing $IMAGE with $IMAGE_WITH_DIGEST"
    sed -i "s|$IMAGE|$IMAGE_WITH_DIGEST|g" "$FILE"
  done
done

if [ "$DEBUG" == true ]; then
  debug "Diff of files updated:"
  diff -r -U 5 "$BUNDLE_DIR" "$PROCESSED_DIR" || true | sed 's|^|        |g'
fi

info "Reusing bundle.Dockerfile from repo in $PROCESSED_DIR"
# Update bundle.Dockerfile to reference processed bundle directories
sed -e "s|COPY deploy/bundle|COPY .|g" "${SCRIPT_DIR}/../bundle.Dockerfile" > "${PROCESSED_DIR}/bundle.Dockerfile"
if [ "$DEBUG" == true ]; then
  debug "Generated bundle.Dockerfile:"
  cat "${PROCESSED_DIR}/bundle.Dockerfile" | sed 's|^|        |g'
fi

if [ "$MULTI_ARCH" == "true" ]; then
  info "Building and pushing bundle $PUSH_IMAGE"
  docker buildx build -t "$PUSH_IMAGE" -f "${PROCESSED_DIR}/bundle.Dockerfile" "$PROCESSED_DIR" \
  --platform "$ARCHITECTURES" \
  --push 2>&1 | sed 's|^|        |g'
else
  info "Building bundle $PUSH_IMAGE"
  $PODMAN build -t "$PUSH_IMAGE" -f "${PROCESSED_DIR}/bundle.Dockerfile" "$PROCESSED_DIR" | sed 's|^|        |g'
  info "Pushing bundle $PUSH_IMAGE"
  $PODMAN push "$PUSH_IMAGE" 2>&1 | sed 's|^|        |g'
fi

NEW_BUNDLE_SHA=$(skopeo inspect "docker://${PUSH_IMAGE}" | jq -r '.Digest')
NEW_BUNDLE_DIGEST="${PUSH_IMAGE%%:*}@${NEW_BUNDLE_SHA}"
info "Resolved image $NEW_BUNDLE_DIGEST from $PUSH_IMAGE"

info "Rendering $NEW_BUNDLE_DIGEST to $RENDER"
BUNDLE_YAML=$(opm render "$NEW_BUNDLE_DIGEST" --output yaml | sed '/---/d')
BUNDLE_NAME="$(echo "$BUNDLE_YAML" | yq -r ".name")"
BUNDLE_FILE=$(readlink -m "${RENDER}/${BUNDLE_NAME}.bundle.yaml")
if [ -f "$BUNDLE_FILE" ]; then
  error "Bundle file $BUNDLE_FILE already exists"
fi
echo "$BUNDLE_YAML" > "$BUNDLE_FILE"
info "Created $BUNDLE_FILE"
