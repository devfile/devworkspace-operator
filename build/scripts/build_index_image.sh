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
  if [ -z $BUNDLE_TAG ]; then
    error "Argument --bundle-tag is required when --release is specified (should be release version)"
  fi
  TAG_REGEX='^v[0-9]+\.[0-9]+\.[0-9]+$'
  if [[ ! "$BUNDLE_TAG" =~ $TAG_REGEX ]]; then
    error "Bundle tag must be specified in the format vX.Y.Z"
  fi
  INDEX_IMAGE="${INDEX_IMAGE:-DEFAULT_RELEASE_INDEX_IMAGE}"
  OUTDIR="olm-catalog/release"
else
  # Set defaults and warn if pushing to main repos in case of accident
  BUNDLE_REPO="${BUNDLE_REPO:-DEFAULT_BUNDLE_REPO}"
  BUNDLE_TAG="${BUNDLE_TAG:-DEFAULT_BUNDLE_TAG}"
  INDEX_IMAGE="${INDEX_IMAGE:-DEFAULT_INDEX_IMAGE}"
  OUTDIR="olm-catalog/next"
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

echo "Building bundle image $BUNDLE_IMAGE"
$PODMAN build . -t "$BUNDLE_IMAGE" -f build/bundle.Dockerfile | sed 's|^|    |'
$PODMAN push "$BUNDLE_IMAGE" 2>&1 | sed 's|^|    |'

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

# Add bundle entry to channel
echo "Adding $BUNDLE_NAME to channel"
if [ "$RELEASE" == "true" ]; then
  # We need to add a replaces and potentially skipRange field to the channel entry
  ENTRIES=$(yq -r '.entries[]?.name' "$CHANNEL_FILE")
  if [ "${BUNDLE_TAG##*.}" != "0" ]; then
    # Bugfix release: replace previous bugfix release for this minor version.
    CURR_MAJOR_MINOR="${BUNDLE_NAME%.*}" # Current release, without bugfix
    PREV_BUGFIX=$(echo "$ENTRIES" | grep "$CURR_MAJOR_MINOR" | sort -V | tail -n 1)
    ENTRY_JSON=$(yq --arg prev_bugfix "$PREV_BUGFIX" '{name, replaces: $prev_bugfix}' "$BUNDLE_FILE")
  else
    # Non-bugfix release: replace previous vX.Y.0 version; skipRange bugfixes for it.
    LATEST_MINOR=$(echo "$ENTRIES" | grep "\.0$" | sort -V | tail -n 1) # latest non-bugfix release (devworkspace-operator.vX.Y.0)
    LATEST_MINOR_BUGFIX=$(echo "$ENTRIES" | grep "${LATEST_MINOR%.0}" | sort -V | tail -n 1) # latest bugfix for this minor release
    SKIP_RANGE=">=${LATEST_MINOR#*.v} <=${LATEST_MINOR_BUGFIX#*.v}" # build version range, dropping 'devworkspace-operator.v' from start
    ENTRY_JSON=$(yq --arg skip_range "$SKIP_RANGE" --arg replaces "$LATEST_MINOR_BUGFIX" '{name, replaces: $replaces, skipRange: $skip_range}' "$BUNDLE_FILE" )
  fi
else
  ENTRY_JSON=$(yq '{name}' "$BUNDLE_FILE")
fi
# shellcheck disable=SC2016
yq -Y -i --argjson entry "$ENTRY_JSON" '.entries |= . + [$entry]' "$CHANNEL_FILE"

echo "Validating current index"
opm validate "$OUTDIR"

# Build index container
echo "Building index image $INDEX_IMAGE"
$PODMAN build . -t "$INDEX_IMAGE" -f build/index.next.Dockerfile | sed 's|^|    |'
$PODMAN push "$INDEX_IMAGE" 2>&1 | sed 's|^|    |'

if [ $DEBUG != "true" ] && [ "$RELEASE" != "true" ]; then
  echo "Cleaning up"
  git restore "$OUTDIR"
  git clean -fd "$OUTDIR"
fi
