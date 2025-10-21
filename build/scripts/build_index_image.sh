#!/bin/bash

set -e

PODMAN=podman
FORCE="false"
MULTI_ARCH="false"
ARCH="amd64"
ARCHITECTURES="linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"
DEBUG="false"

DEFAULT_BUNDLE_REPO="quay.io/devfile/devworkspace-operator-bundle"
DEFAULT_BUNDLE_TAG="next"
DEFAULT_INDEX_IMAGE="quay.io/devfile/devworkspace-operator-index:next"
DEFAULT_RELEASE_INDEX_IMAGE="quay.io/devfile/devworkspace-operator-index:release"
DEFAULT_BUILDER_NAME="dwo-multi-platform-builder"

error() {
  echo "[ERROR] $1"
  exit 1
}

usage() {
cat <<EOF
This script handles building OLM bundle images and adding them to an index.

Usage:
  $0 [args...]

Arguments:
  --release               : Build bundle and index as part of a release (i.e. include an upgrade path
                            from previous versions of the operator). This option overrides defaults below.
                            If specified, the --bundle-tag argument is required, and must be in the format
                            'vX.Y.Z'
                            ** Use of this argument requires all previous releases to be present in **
                            ** olm-catalog/release. This is a manual process                        **
  --bundle-repo <REPO>    : Image repo to use for OLM bundle image (default: $DEFAULT_BUNDLE_REPO)
  --bundle-tag <TAG>      : Image tag to use for the OLM bundle image (default: $DEFAULT_BUNDLE_TAG)
  --index-image <IMAGE>   : Image repo to use for OLM index image (default: $DEFAULT_INDEX_IMAGE)
  --container-tool <TOOL> : Use specific container tool for building/pushing images (default: use podman)
  --force                 : Do not prompt for confirmation if pushing to default repos. Intended for
                            use in CI.
  --multi-arch            : Create images for the following architectures: $ARCHITECTURES. Note: Docker
                            buildx will be used for building and pushing the images, instead of
                            the container tool selected with --container-tool.
  --debug                 : Don't do any normal cleanup on exit, leaving repo in dirty state
  --arch <ARCH>           : The host architecture.
                            This flag is ignored if --multi-arch is used.

 Docker Requirements:
   When using Docker (--container-tool docker), Docker buildx is required for multi-arch builds.
   The script will fail if buildx is not available. Please ensure:
   - Docker version 19.03+ with buildx plugin installed, or
   - Docker Desktop with buildx enabled (default in recent versions)
   For older Docker versions, use --container-tool podman instead.
 
 Examples:
   1. Build and push bundle and index using default image repos
      $0 --force
  2. Build index and bundle using custom images
      $0 --bundle-repo <my_bundle_repo> --bundle-tag dev --index-image <my_index_image>

EOF
}

parse_args() {
  while [[ "$#" -gt 0 ]]; do
    case $1 in
      '--bundle-repo') BUNDLE_REPO="$2"; shift 1;;
      '--bundle-tag') BUNDLE_TAG="$2"; shift 1;;
      '--index-image') INDEX_IMAGE="$2"; shift 1;;
      '--container-tool') PODMAN="$2"; shift 1;;
      '--release') RELEASE="true";;
      '--force') FORCE="true";;
      '--multi-arch') MULTI_ARCH="true";;
      '--debug') DEBUG="true";;
      '--arch') ARCH="$2"; shift 1;;
      *) echo "[ERROR] Unknown parameter is used: $1."; usage; exit 1;;
    esac
    shift 1
  done
}

parse_args "$@"

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

if [ "$RELEASE" == "true" ]; then
  # Set up for release and check arguments
  BUNDLE_REPO="${BUNDLE_REPO:-$DEFAULT_BUNDLE_REPO}"
  if [ -z "$BUNDLE_TAG" ]; then
    error "Argument --bundle-tag is required when --release is specified (should be release version)"
  fi
  TAG_REGEX='^v[0-9]+\.[0-9]+\.[0-9]+$'
  if [[ ! "$BUNDLE_TAG" =~ $TAG_REGEX ]]; then
    error "Bundle tag must be specified in the format vX.Y.Z"
  fi
  INDEX_IMAGE="${INDEX_IMAGE:-$DEFAULT_RELEASE_INDEX_IMAGE}"
  OUTDIR="olm-catalog/release"
  DOCKERFILE="build/index.release.Dockerfile"
else
  # Set defaults and warn if pushing to main repos in case of accident
  BUNDLE_REPO="${BUNDLE_REPO:-$DEFAULT_BUNDLE_REPO}"
  BUNDLE_TAG="${BUNDLE_TAG:-$DEFAULT_BUNDLE_TAG}"
  INDEX_IMAGE="${INDEX_IMAGE:-$DEFAULT_INDEX_IMAGE}"
  OUTDIR="olm-catalog/next"
  DOCKERFILE="build/index.next.Dockerfile"
fi

BUNDLE_IMAGE="${BUNDLE_REPO}:${BUNDLE_TAG}"

# Check we're not accidentally pushing to the DWO repos
if [ "$BUNDLE_REPO" == "quay.io/devfile/devworkspace-operator-bundle" ] && [ "$FORCE" != "true" ]; then
  echo -n "Are you sure you want to push $BUNDLE_IMAGE? [y/N] " && read -r ans && [ "${ans:-N}" = y ] || exit 1
fi
if [ "$INDEX_IMAGE" == "quay.io/devfile/devworkspace-operator-index" ] && [ "$FORCE" != "true" ]; then
  echo -n "Are you sure you want to push $INDEX_IMAGE? [y/N] " && read -r ans && [ "${ans:-N}" = y ] || exit 1
fi

CHANNEL_FILE="$OUTDIR/channel.yaml"

# Set required versions in vars based on current release
CHANNEL_ENTRIES=$(yq -r '.entries[]?.name' "$CHANNEL_FILE")
# LATEST_MINOR_RELEASE is the last-released vX.Y.0 version, e.g. v0.12.0
LATEST_MINOR_RELEASE=$(echo "$CHANNEL_ENTRIES" | grep "\.0$" | sort -V | tail -n 1)
# LATEST_BUGFIX is the last-released vX.Y.Z version, e.g. v0.12.3
LATEST_BUGFIX=$(echo "$CHANNEL_ENTRIES" | grep "${LATEST_MINOR_RELEASE%.0}" | sort -V | tail -n 1)

if [ "$RELEASE" == "true" ]; then
  # shellcheck disable=SC2016
  yq -Yi --arg replaces "$LATEST_BUGFIX" '.spec.replaces = $replaces' \
    deploy/templates/components/csv/clusterserviceversion.yaml
else
  # Remove replaces field from CSVs in nightly builds since the 'next' catalog doesn't currently support
  # an upgrade path.
  yq -Yi 'del(.spec.replaces)' deploy/templates/components/csv/clusterserviceversion.yaml
fi
make generate_olm_bundle_yaml

echo "Building bundle image $BUNDLE_IMAGE"
if [ "$MULTI_ARCH" == "true" ]; then
  docker buildx build . -t "$BUNDLE_IMAGE" -f build/bundle.Dockerfile \
  --platform "$ARCHITECTURES" \
  --push
else
  if [ "$PODMAN" = "docker" ]; then
    # Use Docker for bundle build (disable attestations for OLM compatibility)
    docker build . -t "$BUNDLE_IMAGE" -f build/bundle.Dockerfile --provenance=false
    docker push "$BUNDLE_IMAGE"
  else
    # Use Podman for bundle build
    "$PODMAN" build . -t "$BUNDLE_IMAGE" -f build/bundle.Dockerfile
    "$PODMAN" push "$BUNDLE_IMAGE" 2>&1
  fi
fi


# Get bundle digest using appropriate tool
if [ "$PODMAN" = "docker" ]; then
  # Use Docker to get the digest
  docker pull "${BUNDLE_IMAGE}"
  DOCKER_DIGEST=$(docker inspect "${BUNDLE_IMAGE}" --format '{{index .RepoDigests 0}}')
  BUNDLE_SHA=$(echo "$DOCKER_DIGEST" | cut -d'@' -f2)
else
  # Use skopeo for Podman (with explicit platform to avoid host OS detection issues)
  BUNDLE_SHA=$(skopeo inspect --override-os linux --override-arch "${ARCH}" "docker://${BUNDLE_IMAGE}" | jq -r '.Digest')
fi
BUNDLE_DIGEST="${BUNDLE_REPO}@${BUNDLE_SHA}"
echo "Using bundle image $BUNDLE_DIGEST for index"

# Minor workaround to generate bundle file name from bundle: store file in variable
# temporarily to allow us to inspect it
BUNDLE_YAML=$(opm render "$BUNDLE_DIGEST" --output yaml | sed '/---/d')
BUNDLE_NAME="$(echo "$BUNDLE_YAML" | yq -r ".name")"
BUNDLE_FILE="${OUTDIR}/${BUNDLE_NAME}.bundle.yaml"
if [ -f "$BUNDLE_FILE" ]; then
  error "Bundle file $BUNDLE_FILE already exists"
fi
echo "$BUNDLE_YAML" > "$BUNDLE_FILE"

# Check if bundle with this version is already present in channel
# shellcheck disable=SC2016
if yq -e --arg bundle_name "$BUNDLE_NAME" '[.entries[]?.name] | any(. == $bundle_name)' "$CHANNEL_FILE" >/dev/null; then
  error "Bundle $BUNDLE_NAME is already present in the channel $CHANNEL_FILE"
fi

# Set vars to hold version numbers for relevant bundles -- need to strip of operator name and 'v' prefix from version
# i.e. devworkspace-operator.v0.12.3 -> 0.12.3
BUNDLE_VER="${BUNDLE_NAME##devworkspace-operator.v}"
LATEST_BUGFIX_VER="${LATEST_BUGFIX##devworkspace-operator.v}"
LATEST_MINOR_RELEASE_VER="${LATEST_MINOR_RELEASE##devworkspace-operator.v}"

# Add bundle entry to channel
echo "Adding $BUNDLE_NAME to channel"
if [ "$RELEASE" == "true" ]; then
  # Generate a new channel entry with replaces and  skipRange if a) the current release is a minor version increment
  # and b) previous release had z-stream releases. Otherwise, generate a new channel entry that replaces the last release
  MINOR_REGEX='^[0-9]+\.[0-9]+\.0$'
  if [[ "$BUNDLE_VER" =~ $MINOR_REGEX ]] && [ "$LATEST_BUGFIX_VER" != "$LATEST_MINOR_RELEASE_VER" ]; then
    # shellcheck disable=SC2016
    ENTRY_JSON=$(yq \
      --arg replaces "$LATEST_BUGFIX" \
      --arg skipRange ">=$LATEST_MINOR_RELEASE_VER <$LATEST_BUGFIX_VER" \
      '{name, "replaces": $replaces, "skipRange": $skipRange}' "$BUNDLE_FILE")
  else
    # shellcheck disable=SC2016
    ENTRY_JSON=$(yq \
      --arg replaces "$LATEST_BUGFIX" \
      '{name, "replaces": $replaces}' "$BUNDLE_FILE")
  fi
else
  # Generate a basic entry that replaces all earlier versions
  ENTRY_JSON=$(yq --arg skipRange "<${BUNDLE_VER}" '{name, "skipRange": $skipRange}' "$BUNDLE_FILE")
fi
# shellcheck disable=SC2016
yq -Y -i --argjson entry "$ENTRY_JSON" '.entries |= . + [$entry]' "$CHANNEL_FILE"

echo "Validating current index"
opm validate "$OUTDIR"

# Check if Docker buildx is available and supports the platform
check_buildx_support() {
    if [ "$PODMAN" != "docker" ]; then
        return 1  # Not using Docker, so no buildx
    fi
    
    # Check if buildx command exists
    if ! docker buildx version >/dev/null 2>&1; then
        return 1
    fi
    
    # Check if we can inspect the builder (it's set up and working)
    if ! docker buildx inspect >/dev/null 2>&1; then
        return 1
    fi
    
    return 0
}

# Create and push multi-arch manifest using Podman
create_podman_manifest() {
    local INDEX_IMAGE=$1
    local BUILT_IMAGES=$2
    
    echo "  Using Podman manifest commands"
    
    # Clean up any existing manifest to avoid conflicts
    if "${PODMAN}" manifest inspect "${INDEX_IMAGE}" >/dev/null 2>&1; then
        echo "  Removing existing manifest list"
        "${PODMAN}" manifest rm "${INDEX_IMAGE}" || echo "    (manifest not found, continuing)"
    fi
    
    # Remove any local image with the same name to avoid conflicts
    if "${PODMAN}" image inspect "${INDEX_IMAGE}" >/dev/null 2>&1; then
        echo "  Removing existing local image"
        "${PODMAN}" rmi "${INDEX_IMAGE}" || echo "    (image not found, continuing)"
    fi
    
    echo "  Creating new manifest list with images:$BUILT_IMAGES"
    if ! "${PODMAN}" manifest create "${INDEX_IMAGE}" $BUILT_IMAGES; then
        echo "  ❌ Failed to create Podman manifest list"
        exit 1
    fi
    
    echo "  Pushing manifest list"
    if ! "${PODMAN}" manifest push "${INDEX_IMAGE}"; then
        echo "  ❌ Failed to push Podman manifest list"
        exit 1
    fi
    
    echo "  ✅ Successfully pushed Podman manifest list"
}

# Build and push using Docker buildx (multi-platform capable)
build_with_docker_buildx() {
    local INDEX_IMAGE=$1
    local DOCKERFILE=$2
    
    echo "Building multi-arch image with Docker buildx"
    echo "  Building ${INDEX_IMAGE} for linux/amd64,linux/arm64"
    
    if ! docker buildx build --platform linux/amd64,linux/arm64 \
        -t "${INDEX_IMAGE}" \
        -f "${DOCKERFILE}" . \
        --provenance=false \
        --push; then
        echo "  ❌ Failed to build and push ${INDEX_IMAGE}"
        exit 1
    fi
    
    echo "  ✅ Successfully built and pushed multi-arch image ${INDEX_IMAGE}"
}


# Build and push using Podman (multi-platform capable)
build_with_podman() {
    local BASE_IMAGE=$1
    local DOCKERFILE=$2
    
    echo "Building multi-arch images with Podman"
    
    for TARGET_ARCH in "amd64" "arm64"; do
        local ARCH_IMAGE="${BASE_IMAGE}-${TARGET_ARCH}"
        
        # Determine the appropriate Dockerfile based on build type (native vs cross-build)
        local BUILD_DOCKERFILE
        if [ "${ARCH}" == "${TARGET_ARCH}" ]; then
            # NATIVE BUILD: Pre-generate the cache - use the main dockerfile
            echo "  Building ${ARCH_IMAGE} for linux/${TARGET_ARCH} (native build with cache)"
            BUILD_DOCKERFILE="$DOCKERFILE"
        else
            # CROSS-BUILD: Build cache at runtime - use the no-cache variant
            echo "  Building ${ARCH_IMAGE} for linux/${TARGET_ARCH} (cross-build, no cache)"
            BUILD_DOCKERFILE="${DOCKERFILE%.Dockerfile}.no-cache.Dockerfile"
        fi
        
        echo "    Using dockerfile: ${BUILD_DOCKERFILE}"
        if ! "${PODMAN}" build --platform "linux/${TARGET_ARCH}" \
            -t "${ARCH_IMAGE}" \
            -f "${BUILD_DOCKERFILE}" .; then
            echo "  ❌ Failed to build ${ARCH_IMAGE}"
            exit 1
        fi
        
        echo "  Pushing ${ARCH_IMAGE}"
        if ! "${PODMAN}" push "${ARCH_IMAGE}"; then
            echo "  ❌ Failed to push ${ARCH_IMAGE}"
            exit 1
        fi
        
        echo "  ✅ Successfully built and pushed ${ARCH_IMAGE}"
    done
    
}


# Build index container
echo "Building index image $INDEX_IMAGE"
if [ "$MULTI_ARCH" == "true" ]; then
  docker buildx build . -t "$INDEX_IMAGE" -f "$DOCKERFILE" \
  --platform "$ARCHITECTURES" \
  --push
else
  # Build images using the appropriate method
  echo ""
  
  if [ "$PODMAN" = "docker" ]; then
    # Require Docker buildx for multi-arch builds
    if ! check_buildx_support; then
      error "Docker buildx is required but not available. Please update Docker or enable buildx."
    fi
    echo "Using Docker buildx for multi-arch builds"
    build_with_docker_buildx "$INDEX_IMAGE" "$DOCKERFILE"
    echo "✅ Published multi-arch image: $INDEX_IMAGE"
  else
    echo "Using Podman for multi-arch builds"
    build_with_podman "$INDEX_IMAGE" "$DOCKERFILE"
    
    # Since Podman builds individual arch images, we need to create a manifest
    BUILT_IMAGES="${INDEX_IMAGE}-amd64 ${INDEX_IMAGE}-arm64"
    
    echo ""
    echo "Multi-arch build successful, creating manifest list"
    echo "Built images:$BUILT_IMAGES"
    
    create_podman_manifest "$INDEX_IMAGE" "$BUILT_IMAGES"
    echo "✅ Published multi-arch manifest: $INDEX_IMAGE"
  fi
fi


if [ $DEBUG != "true" ] && [ "$RELEASE" != "true" ]; then
  echo "Cleaning up"
  git restore "$OUTDIR"
  git clean -fd "$OUTDIR"
fi
