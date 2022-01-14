//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

import (
	"os"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

const testNamespace = "test-namespace"

var (
	scheme   = runtime.NewScheme()
	trueBool = true
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(routev1.Install(scheme))
	utilruntime.Must(configv1.Install(scheme))
}

func setupForTest(t *testing.T) {
	if err := os.Setenv("WATCH_NAMESPACE", testNamespace); err != nil {
		t.Fatalf("failed to set up for test: %s", err)
	}
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	configNamespace = testNamespace
	originalDefaultConfig := DefaultConfig.DeepCopy()
	t.Cleanup(func() {
		internalConfig = nil
		DefaultConfig = originalDefaultConfig
	})
}

func buildConfig(config *v1alpha1.OperatorConfiguration) *v1alpha1.DevWorkspaceOperatorConfig {
	return &v1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorConfigName,
			Namespace: testNamespace,
		},
		Config: config,
	}
}
