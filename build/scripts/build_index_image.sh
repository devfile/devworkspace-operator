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
LATEST_MINOR_RELEASE=$(echo "$CHANNEL_ENTRIES" | grep "\.0$" | sort -V | tail -n 1)
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
  # Generate a new channel entry that replaces the last release
  # TODO: Also generate a skipRange so that e.g. v0.10.0 replaces v0.9.3 and skips v0.9.0-v0.9.2
  # shellcheck disable=SC2016
  ENTRY_JSON=$(yq --arg replaces "$LATEST_BUGFIX" '{name, replaces: $replaces}' "$BUNDLE_FILE")
else
  # Generate a basic entry with no upgrade path for nightly releases
  ENTRY_JSON=$(yq '{name}' "$BUNDLE_FILE")
fi
# shellcheck disable=SC2016
yq -Y -i --argjson entry "$ENTRY_JSON" '.entries |= . + [$entry]' "$CHANNEL_FILE"

echo "Validating current index"
opm validate "$OUTDIR"

# Build index container
# Building file-based-catalogs with OPM is broken in v1.19.1 due to https://github.com/operator-framework/operator-registry/pull/807
# Instead, use previous opm v1.18.z process
# $PODMAN build . -t "$INDEX_IMAGE" -f "$DOCKERFILE" | sed 's|^|    |'
# $PODMAN push "$INDEX_IMAGE" 2>&1 | sed 's|^|    |'
echo "Building index image $INDEX_IMAGE"
if [ "$RELEASE" == "true" ]; then
  # The command below makes this script fragile. The index image represents the global (i.e. across all releases)
  # state of the operator update graph so if the new bundle doesn't slot cleanly into what exists, this will fail.
  opm index add \
    --bundles "$BUNDLE_DIGEST" \
    --from-index "$INDEX_IMAGE" \
    --tag "$INDEX_IMAGE" \
    --container-tool "$PODMAN"
else
  opm index add \
    --bundles "$BUNDLE_DIGEST" \
    --tag "$INDEX_IMAGE" \
    --container-tool "$PODMAN"
fi
$PODMAN push "$INDEX_IMAGE" 2>&1 | sed 's|^|    |'

if [ $DEBUG != "true" ] && [ "$RELEASE" != "true" ]; then
  echo "Cleaning up"
  git restore "$OUTDIR"
  git clean -fd "$OUTDIR"
fi
