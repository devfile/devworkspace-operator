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

REGEX="^([0-9]+)\\.([0-9]+)\\.([0-9]+)(\\-[0-9a-z-]+(\\.[0-9a-z-]+)*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

CURRENT_DIR=$(pwd)
BASE_DIR=$(cd "$(dirname "$0")"; pwd)
source ${BASE_DIR}/check-yq.sh

if [[ "$1" =~ $REGEX ]]
then
  RELEASE="$1"
else
  echo "You should provide the new release as the first parameter"
  echo "and it should be semver-compatible with optional *lower-case* pre-release part"
  exit 1
fi

create_copy_diff_crds() {
  cp "${BASE_DIR}/../deploy/crds/workspace.che.eclipse.org_$1_crd.yaml" "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml"
  diff -u "${packageFolderPath}/${lastPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml" \
  "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml" \
  > "${packageFolderPath}/${newNightlyPackageVersion}/eclipse-che-preview-${platform}-$1.crd.yaml.diff" || true
  echo "   - Updating the 'stable' channel with new version in the package descriptor: ${packageFilePath}"
  sed -e "s/${lastPackageVersion}/${newNightlyPackageVersion}/" "${packageFilePath}" > "${packageFilePath}.new"
  mv "${packageFilePath}.new" "${packageFilePath}"
}

for platform in 'kubernetes' 'openshift'
do
  packageName="eclipse-che-preview-${platform}"
  echo
  echo "## Creating release '${RELEASE}' of the OperatorHub package '${packageName}' for platform '${platform}'"

  packageBaseFolderPath="${BASE_DIR}/${packageName}"
  cd "${packageBaseFolderPath}"

  packageFolderPath="${packageBaseFolderPath}/deploy/olm-catalog/${packageName}"
  packageFilePath="${packageFolderPath}/${packageName}.package.yaml"
  lastPackageNightlyVersion=$(yq -r '.channels[] | select(.name == "nightly") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  lastPackagePreReleaseVersion=$(yq -r '.channels[] | select(.name == "stable") | .currentCSV' "${packageFilePath}" | sed -e "s/${packageName}.v//")
  echo "   - Last package nightly version: ${lastPackageNightlyVersion}"
  echo "   - Last package pre-release version: ${lastPackagePreReleaseVersion}"
  if [ "${lastPackagePreReleaseVersion}" == "${RELEASE}" ]
  then
    echo "Release ${RELEASE} already exists in the package !"
    echo "You should first remove it"
    exit 1
  fi

  echo "     => will create release '${RELEASE}' from nightly version '${lastPackageNightlyVersion}' that will replace previous release '${lastPackagePreReleaseVersion}'"

  mkdir -p "${packageFolderPath}/${RELEASE}"

  echo "   - Copying the CRD file"
  # Components crd
  create_copy_diff_crds "components"

  # Workspaceroutings crd
  create_copy_diff_crds "workspaceroutings"

  # Workspace crd
  create_copy_diff_crds "workspaces"

  diff -u "${packageFolderPath}/${lastPackagePreReleaseVersion}/${packageName}.v${lastPackagePreReleaseVersion}.clusterserviceversion.yaml" \
  "${packageFolderPath}/${RELEASE}/${packageName}.v${RELEASE}.clusterserviceversion.yaml" \
  > "${packageFolderPath}/${RELEASE}/${packageName}.v${RELEASE}.clusterserviceversion.yaml.diff" || true
done
cd "${CURRENT_DIR}"
