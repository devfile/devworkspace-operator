//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package cluster

import (
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// Create a new openshift client using the in cluster config
func openShiftClient() (*configv1client.ConfigV1Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	if config != nil {
		client, err := configv1client.NewForConfig(config)
		if err != nil {
			return client, err
		}
		return client, nil
	}
	return nil, nil
}

// Get the version of the current running OpenShift cluster in semver format E.g. 4.5.9
func OpenshiftVersion() (string, error) {
	client, err := openShiftClient()
	if err != nil {
		return "", err
	}

	openshiftAPIServer, err := client.ClusterOperators().Get("openshift-apiserver", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if openshiftAPIServer != nil {
		for _, ver := range openshiftAPIServer.Status.Versions {
			if ver.Name == "operator" {
				// Apparently only openshift-apiserver clusteroperator reports the version number
				return ver.Version, nil
			}
		}
	}
	return "", nil
}
