#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

source olm.sh

installOperatorMarketPlace
installPackage
applyCRCheCluster
waitCheServerDeploy

echo -e "\u001b[32m Installation of the che-operator version: ${CSV} succesfully completed \u001b[0m"
