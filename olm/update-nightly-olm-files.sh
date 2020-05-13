#!/bin/bash
#
# Copyright (c) 2019 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
source ${BASE_DIR}/check-yq.sh

create_copy_diff_crds() {
  cp "${BASE_DIR}/../deploy/crds/workspace.che.eclipse.org_$1_crd.yaml" "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml"
  diff -u "${packageFolderPath}/${lastPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml" \
  "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml" \
  > "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml.diff" || true
  echo "   - Updating the 'nightly' channel with new version in the package descriptor: ${packageFilePath}"
  sed -e "s/${lastPackageVersion}/${newNightlyPackageVersion}/" "${packageFilePath}" > "${packageFilePath}.new"
  mv "${packageFilePath}.new" "${packageFilePath}"
}

for platform in 'kubernetes' 'openshift'
do
  packageName=eclipse-che-preview-${platform}
  echo
  echo "## Updating OperatorHub package '${packageName}' for platform '${platform}'"
  packageBaseFolderPath=${BASE_DIR}/${packageName}
  cd "${packageBaseFolderPath}"
  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  lastPackageVersion=$(yq -r '.channels[] | select(.name == "nightly") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  echo "   - Last package version: ${lastPackageVersion}"
  newNightlyPackageVersion="9.9.9-nightly.$(date +%s)"
  echo "     => will create a new version: ${newNightlyPackageVersion}"
  ./build-roles.sh
  echo "   - Updating new package version with roles defined in: ${role}"
  operator-sdk olm-catalog gen-csv --csv-version "${newNightlyPackageVersion}" --from-version="${lastPackageVersion}" 2>&1 | sed -e 's/^/      /'
  containerImage=$(sed -n 's|^ *image: *\([^ ]*/che-workspace-operator:[^ ]*\) *|\1|p' "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml")
  createdAt=$(date -u +%FT%TZ)
  echo "   - Updating new package version fields:"
  echo "       - containerImage => ${containerImage}" 
  echo "       - createdAt => ${createdAt}" 
  sed \
  -e "s|containerImage:.*$|containerImage: ${containerImage}|" \
  -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" \
  "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml" \
  > "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml.new"
  mv "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml.new" \
  "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml"
  echo "   - Copying the CRD file"

  # Components crd
  create_copy_diff_crds "components"

  # Workspaceroutings crd
  create_copy_diff_crds "workspaceroutings"

  # Workspace crd
  create_copy_diff_crds "workspaces"

  diff -u "${packageFolderPath}/${lastPackageVersion}/${packageName}.v${lastPackageVersion}.clusterserviceversion.yaml" \
  "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml" \
  > "${packageFolderPath}/${newNightlyPackageVersion}/${packageName}.v${newNightlyPackageVersion}.clusterserviceversion.yaml.diff" || true
done
cd "${CURRENT_DIR}"