#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# This script builds complete deployment files for the DevWorkspace Operator,
# filling all environment variables as appropriate. The output, stored in
# deploy/deployment, contains subfolders for OpenShift and Kubernetes. Within each
# is a file, combined.yaml, which stores all the objects involved in deploying
# the operator, and a subfolder, objects, which stores separate yaml files for
# each object in combined.yaml, with the name <object-name>.<object-kind>.yaml
#
# Accepts parameter `--use-defaults`, which will generate static files based on
# default environment variables. Otherwise, current environment variables are
# respected.
#
# Note: The configmap generated when using `--use-defaults` will have an empty
# value for devworkspace.routing.cluster_host_suffix as there is no suitable
# default.

set -e

# List of environment variables that will be replaced by envsubst
SUBST_VARS='$NAMESPACE $DWO_IMG $RBAC_PROXY_IMAGE $PROJECT_CLONE_IMG $ROUTING_SUFFIX $DEFAULT_ROUTING $PULL_POLICY'

SCRIPT_DIR=$(cd "$(dirname "$0")"; pwd)

function print_help() {
  cat << EOF
Usage: generate-deployment.sh [ARGS]
Arguments:
  --use-defaults
      Output deployment files to deploy/deployment, using default
      environment variables rather than current shell variables.
      Implies '--split yaml'
  --default-image
      Controller (and webhook) image to use for the default deployment.
      Used only when '--use-defaults' is passed; otherwise, the value of
      the DWO_IMG environment variable is used. If unspecified, the default
      value of 'quay.io/devfile/devworkspace-controller:next' is used
  --project-clone-image
      Image to use for the project clone init container. Used only when
      '--use-defaults' is passed; otherwise, the value of the PROJECT_CLONE_IMG
      environment variable is used. If unspecifed, the default value of
      'quay.io/devfile/project-clone:next' is used.
  --split-yaml
      Parse output file combined.yaml into a yaml file for each record
      in combined yaml. Files are output to the 'objects' subdirectory
      for each platform and are named <object-name>.<kind>.yaml
  --generate-olm
      Generate deployment files to be consumed by operator-sdk in creating
      a bundle. If this option is set, --use-defaults is set as well (i.e.
      output directory is always deploy/deployment)
  -h, --help
      Print this help description
EOF
}

USE_DEFAULT_ENV=false
GEN_OLM=false
OUTPUT_DIR="${SCRIPT_DIR%/}/current"
SPLIT_YAMLS=false
while [[ "$#" -gt 0 ]]; do
  case $1 in
      --use-defaults)
      USE_DEFAULT_ENV=true
      SPLIT_YAMLS=true
      OUTPUT_DIR="${SCRIPT_DIR%/}/deployment"
      ;;
      --generate-olm)
      GEN_OLM=true
      USE_DEFAULT_ENV=true
      SPLIT_YAMLS=true
      OUTPUT_DIR="${SCRIPT_DIR%/}/deployment"
      ;;
      --default-image)
      DEFAULT_IMAGE=$2
      shift
      ;;
      --project-clone-image)
      PROJECT_CLONE_IMG=$2
      shift
      ;;
      --split-yamls)
      SPLIT_YAMLS=true
      ;;

      -h|--help)
      print_help
      exit 0
      ;;
      *)
      echo "Unknown parameter passed: $1"
      print_help
      exit 1
      ;;
  esac
  shift
done

if $USE_DEFAULT_ENV; then
  echo "Using defaults for environment variables"
  export NAMESPACE=devworkspace-controller
  export DWO_IMG=${DEFAULT_IMAGE:-"quay.io/devfile/devworkspace-controller:next"}
  export PROJECT_CLONE_IMG=${PROJECT_CLONE_IMG:-"quay.io/devfile/project-clone:next"}
  export PULL_POLICY=Always
  export DEFAULT_ROUTING=basic
  export DEVWORKSPACE_API_VERSION=03e023e7078b64884216d8e6dce8f0cf8b7e74d2
  export ROUTING_SUFFIX='""'
  export FORCE_DEVWORKSPACE_CRDS_UPDATE=true
fi

KUBERNETES_DIR="${OUTPUT_DIR}/kubernetes"
OPENSHIFT_DIR="${OUTPUT_DIR}/openshift"
OLM_DIR="${OUTPUT_DIR}/olm"
COMBINED_FILENAME="combined.yaml"
OBJECTS_DIR="objects"

KUSTOMIZE_VER=4.0.5
KUSTOMIZE_DIR="${SCRIPT_DIR}/../bin/kustomize"
KUSTOMIZE=${KUSTOMIZE_DIR}/kustomize

rm -rf "$KUBERNETES_DIR" "$OPENSHIFT_DIR"
mkdir -p "$KUBERNETES_DIR" "$OPENSHIFT_DIR" "$OLM_DIR"

required_vars=(NAMESPACE DWO_IMG PULL_POLICY DEFAULT_ROUTING \
  DEVWORKSPACE_API_VERSION)
for var in "${required_vars[@]}"; do
  if [ -z "${!var}" ]; then
    echo "ERROR: Environment variable $var must be set"
    exit 1
  fi
done

required_bin=(envsubst csplit yq)
for bin in "${required_bin[@]}"; do
  if ! which "$bin" &>/dev/null; then
    echo "ERROR: Program $bin is required for this script"
  fi
done

if [ -n "${FORCE_DEVWORKSPACE_CRDS_UPDATE}" ]; then
  # Ensure we have correct version of devfile/api CRDs
  echo "Updating devfile/api CRDs"
  ./update_devworkspace_crds.sh --api-version "$DEVWORKSPACE_API_VERSION"
else
  # Make sure devfile/api CRDs are present but do not force update
  echo "Checking for devfile/api CRDs"
  ./update_devworkspace_crds.sh --init --api-version "$DEVWORKSPACE_API_VERSION"
fi

mkdir -p "$KUSTOMIZE_DIR"
if [ ! -f "$KUSTOMIZE" ]; then
  curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" \
    | bash -s "$KUSTOMIZE_VER" "$KUSTOMIZE_DIR"
elif [ "$($KUSTOMIZE version | grep -o 'Version:[^ ]*')" != "Version:kustomize/v${KUSTOMIZE_VER}" ]; then
  echo "Wrong version of kustomize at ${KUSTOMIZE}. Redownloading."
  rm "$KUSTOMIZE"
  curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" \
    | bash -s "$KUSTOMIZE_VER" "$KUSTOMIZE_DIR"
fi

# Run kustomize to build yamls
echo "Generating config for Kubernetes"
export RBAC_PROXY_IMAGE="${KUBE_RBAC_PROXY_IMAGE:-gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0}"
${KUSTOMIZE} build "${SCRIPT_DIR}/templates/cert-manager" \
  | envsubst "$SUBST_VARS" \
  > "${KUBERNETES_DIR}/${COMBINED_FILENAME}"
unset RBAC_PROXY_IMAGE
echo "File saved to ${KUBERNETES_DIR}/${COMBINED_FILENAME}"

echo "Generating config for OpenShift"
export RBAC_PROXY_IMAGE="${OPENSHIFT_RBAC_PROXY_IMAGE:-registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.8}"
${KUSTOMIZE} build "${SCRIPT_DIR}/templates/service-ca" \
  | envsubst "$SUBST_VARS" \
  > "${OPENSHIFT_DIR}/${COMBINED_FILENAME}"
unset RBAC_PROXY_IMAGE
echo "File saved to ${OPENSHIFT_DIR}/${COMBINED_FILENAME}"

if $GEN_OLM; then
  echo "Generating base deployment files for OLM"
  export RBAC_PROXY_IMAGE="${OPENSHIFT_RBAC_PROXY_IMAGE:-registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.8}"
  export NAMESPACE=openshift-operators
  ${KUSTOMIZE} build "${SCRIPT_DIR}/templates/olm" \
    | envsubst "$SUBST_VARS" \
    > "${OLM_DIR}/${COMBINED_FILENAME}"
  unset RBAC_PROXY_IMAGE
  echo "File saved to ${OLM_DIR}/${COMBINED_FILENAME}"
fi

if ! $SPLIT_YAMLS; then
  echo "Skipping split combined.yaml step. To split the combined yaml, use the --split-yamls argument."
  exit 0
fi

if ! command -v yq &>/dev/null; then
  echo "Program yq is required for this step; please install it via 'pip install yq'"
  exit 1
fi

# Split the giant files output by kustomize per-object
# Do not split the OLM files as the `operator-sdk generate bundle` will generate
# duplicate files when using $OLM_DIR as a --deploy-dir
for dir in "$KUBERNETES_DIR" "$OPENSHIFT_DIR"; do
  echo "Parsing objects from ${dir}/${COMBINED_FILENAME}"
  mkdir -p "$dir/$OBJECTS_DIR"
  # Have to move into subdirectory as csplit outputs to the current working dir
  pushd "$dir" &>/dev/null
  # Split combined.yaml into separate files for each record, with names temp01,
  # temp02, etc. Then rename each temp file according to the .metadata.name and
  # .kind of the object
  csplit -s -f "temp" --suppress-matched "${dir}/combined.yaml" '/^---$/' '{*}'
  for file in temp??; do
    name_kind=$(yq -r '"\(.metadata.name).\(.kind)"' "$file")
    mv "$file" "objects/${name_kind}.yaml"
  done
  popd &>/dev/null
done
