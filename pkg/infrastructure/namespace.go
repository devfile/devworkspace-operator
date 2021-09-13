//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package infrastructure

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	WatchNamespaceEnvVar = "WATCH_NAMESPACE"
)

// GetOperatorNamespace returns the namespace the operator should be running in.
//
// This function was ported over from Operator SDK 0.17.0 and modified.
func GetOperatorNamespace() (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("could not read namespace from mounted serviceaccount info")
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

// GetWatchNamespace returns the namespace the operator should be watching for changes
//
// This function was ported over from Operator SDK 0.17.0
func GetWatchNamespace() (string, error) {
	ns, found := os.LookupEnv(WatchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", WatchNamespaceEnvVar)
	}
	return ns, nil
}

// GetNamespace gets the namespace of the operator by checking GetOperatorNamespace and GetWatchNamespace in that order.
// Returns an error if both GetOperatorNamespace and GetWatchNamespace return an error.
func GetNamespace() (string, error) {
	ns, operErr := GetOperatorNamespace()
	if operErr == nil {
		return ns, nil
	}
	ns, watchErr := GetWatchNamespace()
	if watchErr != nil {
		return "", fmt.Errorf("failed to get current namespace: %s; %s", operErr, watchErr)
	}
	return ns, nil
}
