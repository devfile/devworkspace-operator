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

# strip_tag REF prints REF without its tag, leaving the registry port and any digest alone.
strip_tag() {
  local ref="${1%@*}"
  local last="${ref##*/}"
  if [[ "$last" == *:* ]]; then
    echo "${ref%:*}"
  else
    echo "$ref"
  fi
}

# check_pinnable REF fails when REF cannot be pinned with an OpenShift PinnedImageSet: the
# reference must be a digest accepted by the API, and the machine config daemon runs
# `podman manifest inspect` on it, which rejects a single OCI image manifest with rc=125.
#
# The NAME_RE and the length limit mirror the CEL rules on spec.pinnedImages[*].name. They are
# deliberately duplicated from IMAGE_RE in
# che-operator/build/scripts/release/checkPinnedImages.sh: this release must not fetch the
# checker, so keep the two in sync by hand.
check_pinnable() {
  local ref="$1"
  local output
  local name_re='^([a-zA-Z0-9-]+\.)+[a-zA-Z0-9-]+(:[0-9]{2,5})?/([a-zA-Z0-9_-]{0,61}/)?[a-zA-Z0-9_.-]*@sha256:[a-f0-9]{64}$'
  local max_reference_length=447

  if (( ${#ref} > max_reference_length )); then
    error "$ref is longer than the ${max_reference_length} characters the PinnedImageSet API accepts"
  fi

  if [[ ! "$ref" =~ $name_re ]]; then
    error "$ref is not a digest reference accepted by the PinnedImageSet API"
  fi

  # An empty store, because an image already present locally would be inspected offline.
  if ! output=$(podman --log-level error --root "$PINNED_IMAGE_STORE/root" --runroot "$PINNED_IMAGE_STORE/run" \
    manifest inspect "$ref" 2>&1); then
    # Keep the cause: a dead digest, missing credentials, a TLS problem and a single OCI
    # manifest all fail here, and they call for different actions. The single-manifest error
    # embeds the whole manifest JSON with the cause at the end, so keep head and tail.
    output="${output//$'\n'/ }"
    (( ${#output} > 300 )) && output="${output:0:90} ... ${output: -180}"
    error "$ref cannot be pre-pulled: podman manifest inspect failed: ${output}"
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
trap 'rm -rf "$TMPDIR"' EXIT
info "Working in $TMPDIR"

# An empty container store for the pinnability check, because an image already present locally
# would be inspected offline and hide a bad reference.
PINNED_IMAGE_STORE="$TMPDIR/pinned-image-store"
mkdir -p "$PINNED_IMAGE_STORE/root" "$PINNED_IMAGE_STORE/run"
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
    if [[ -z "$DIGEST" || "$DIGEST" == "null" ]]; then
      error "Failed to get the digest of $IMAGE"
    fi
    IMAGE_WITH_DIGEST="$(strip_tag "$IMAGE")@${DIGEST}"
    info "Replacing $IMAGE with $IMAGE_WITH_DIGEST"
    sed -i "s|$IMAGE|$IMAGE_WITH_DIGEST|g" "$FILE"
  done

  # The operator passes these to the workspace, so they reach administrators' PinnedImageSets.
  info "Checking that every RELATED_IMAGE_* value of $FILE can be pinned"
  INVALID=$(yq -r '[.spec.install.spec.deployments[]?.spec.template.spec | (.containers[]?, .initContainers[]?) | .env[]?
                 | select(.name | test("^RELATED_IMAGE_"))
                 | select((.value // "") == "" or (.value | test("\n")))] | length' "$FILE") ||
    error "Failed to read the RELATED_IMAGE_* values of $FILE"
  if [[ "$INVALID" != "0" ]]; then
    error "Empty or multi-line RELATED_IMAGE_* value in $FILE"
  fi
  REFS=$(yq -r '.spec.install.spec.deployments[]?.spec.template.spec | (.containers[]?, .initContainers[]?) | .env[]?
                 | select(.name | test("^RELATED_IMAGE_")) | .value' "$FILE") ||
    error "Failed to read the RELATED_IMAGE_* values of $FILE"
  if [[ -z "$REFS" ]]; then
    error "No RELATED_IMAGE_* values found in $FILE"
  fi
  while IFS= read -r REF; do
    if [[ -z "$REF" ]]; then
      error "Empty RELATED_IMAGE_* value in $FILE"
    fi
    check_pinnable "$REF"
  done <<< "$REFS"
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
NEW_BUNDLE_DIGEST="$(strip_tag "$PUSH_IMAGE")@${NEW_BUNDLE_SHA}"
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
