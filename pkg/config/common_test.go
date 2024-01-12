//
// Copyright (c) 2019-2024 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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

const (
	testNamespace           = "test-namespace"
	externalConfigName      = "external-config-name"
	externalConfigNamespace = "external-config-namespace"
)

var (
	scheme = runtime.NewScheme()
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
	setDefaultPodSecurityContext()
	setDefaultContainerSecurityContext()
	configNamespace = testNamespace
	originalDefaultConfig := defaultConfig.DeepCopy()
	t.Cleanup(func() {
		internalConfig = nil
		defaultConfig = originalDefaultConfig
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

func buildExternalConfig(config *v1alpha1.OperatorConfiguration) *v1alpha1.DevWorkspaceOperatorConfig {
	return &v1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      externalConfigName,
			Namespace: externalConfigNamespace,
		},
		Config: config,
	}
}
