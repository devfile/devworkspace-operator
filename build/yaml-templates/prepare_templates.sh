#!/bin/bash

set -e

if [ -z "$DEVWORKSPACE_BRANCH" ]; then
  echo "Environment variable DEVWORKSPACE_BRANCH must be set"
  exit 1
fi

BASE_DIR=${1:-/build}
OUTPUT_DIR="${BASE_DIR}/deploy"
TARBALL_PATH="${BASE_DIR}/devworkspace_operator_templates.tar.gz"

# Clone repo
git clone --depth=1 --branch "${DEVWORKSPACE_BRANCH}" https://github.com/devfile/devworkspace-operator.git
cd devworkspace-operator

# Grab devfile/api CRDs
./update_devworkspace_crds.sh --init --api-version "$DEVWORKSPACE_API_VERSION"

# Fill env in template files
mv config/cert-manager/kustomization.yaml config/cert-manager/kustomization.yaml.bak
mv config/service-ca/kustomization.yaml config/service-ca/kustomization.yaml.bak
mv config/base/config.properties config/base/config.properties.bak
mv config/base/manager_image_patch.yaml config/base/manager_image_patch.yaml.bak
envsubst < config/cert-manager/kustomization.yaml.bak > config/cert-manager/kustomization.yaml
envsubst < config/service-ca/kustomization.yaml.bak > config/service-ca/kustomization.yaml
envsubst < config/base/config.properties.bak > config/base/config.properties
envsubst < config/base/manager_image_patch.yaml.bak > config/base/manager_image_patch.yaml

# Generate yaml files
mkdir -p "${OUTPUT_DIR}"/{kubernetes,openshift}
echo "===== Building yaml templates for Kubernetes ====="
kustomize build config/cert-manager > "${OUTPUT_DIR}/kubernetes/combined.yaml"
echo "===== Generated yaml templates for Kubernetes ====="
echo "===== Building yaml templates for OpenShift ====="
kustomize build config/service-ca > "${OUTPUT_DIR}/openshift/combined.yaml"
echo "===== Generated yaml templates for OpenShift ====="

# Take giant yaml file output by kustomize and separate it into a file per k8s object
for dir in kubernetes openshift; do
  echo "===== Parsing files from ${OUTPUT_DIR}/${dir}/combined.yaml ====="
  pushd "${OUTPUT_DIR}/${dir}" &>/dev/null
  mkdir -p objects

  # Split combined.yaml into separate files for each record, with names temp01, temp02, etc.
  # Then rename each temp file according to the .metadata.name and .kind of the object
  csplit -s -f "temp" --suppress-matched "combined.yaml" '/^---$/' '{*}'
  for file in temp*; do
    name_kind=$(yq -r '"\(.metadata.name).\(.kind)"' "$file")
    mv "$file" "objects/${name_kind}.yaml"
  done

  popd &>/dev/null
done

# Compress files into a tarball
cd "$BASE_DIR"
tar -czvf "$TARBALL_PATH" deploy
