#!/bin/bash

set -e

PODMAN=podman
FORCE="false"
DEBUG="false"

DEFAULT_BUNDLE_REPO="quay.io/devfile/devworkspace-operator-bundle"
DEFAULT_BUNDLE_TAG="next"
DEFAULT_INDEX_IMAGE="quay.io/devfile/devworkspace-operator-index:next"
DEFAULT_RELEASE_INDEX_IMAGE="quay.io/devfile/devworkspace-operator-index:release"

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
  --debug                 : Don't do any normal cleanup on exit, leaving repo in dirty state

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
      '--debug') DEBUG="true";;
      *) echo "[ERROR] Unknown parameter is used: $1."; usage; exit 1;;
    esac
    shift 1
  done
}

parse_args "$@"

if [ "$RELEASE" == "true" ]; then
  # Set up for release and check arguments
  BUNDLE_REPO="${BUNDLE_REPO:-DEFAULT_BUNDLE_REPO}"
  if [ -z "$BUNDLE_TAG" ]; then
    error "Argument --bundle-tag is required when --release is specified (should be release version)"
  fi
  TAG_REGEX='^v[0-9]+\.[0-9]+\.[0-9]+$'
  if [[ ! "$BUNDLE_TAG" =~ $TAG_REGEX ]]; then
    error "Bundle tag must be specified in the format vX.Y.Z"
  fi
  INDEX_IMAGE="${INDEX_IMAGE:-DEFAULT_RELEASE_INDEX_IMAGE}"
  OUTDIR="olm-catalog/release"
  DOCKERFILE="build/index.release.Dockerfile"
else
  # Set defaults and warn if pushing to main repos in case of accident
  BUNDLE_REPO="${BUNDLE_REPO:-DEFAULT_BUNDLE_REPO}"
  BUNDLE_TAG="${BUNDLE_TAG:-DEFAULT_BUNDLE_TAG}"
  INDEX_IMAGE="${INDEX_IMAGE:-DEFAULT_INDEX_IMAGE}"
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

echo "Setting up buildx builder"
docker buildx create --name multiplatformbuilder --use

echo "Building bundle image $BUNDLE_IMAGE"
#$PODMAN build . -t "$BUNDLE_IMAGE" -f build/bundle.Dockerfile
docker buildx build . --platform linux/amd64,linux/arm64,linux/ppc64le,linux/s390x -t "$BUNDLE_IMAGE"  -f build/bundle.Dockerfile
nostalgic_brown
$PODMAN push "$BUNDLE_IMAGE" 2>&1

BUNDLE_SHA=$(skopeo inspect "docker://${BUNDLE_IMAGE}" | jq -r '.Digest')
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

# Build index container
echo "Building index image $INDEX_IMAGE"
#$PODMAN build . -t "$INDEX_IMAGE" -f "$DOCKERFILE"

docker buildx build . --platform linux/amd64,linux/arm64,linux/ppc64le,linux/s390x -t "$INDEX_IMAGE"  -f "$DOCKERFILE"

$PODMAN push "$INDEX_IMAGE" 2>&1

if [ $DEBUG != "true" ] && [ "$RELEASE" != "true" ]; then
  echo "Cleaning up"
  git restore "$OUTDIR"
  git clean -fd "$OUTDIR"
fi
