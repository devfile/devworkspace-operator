//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

package workspace

import (
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
)

func TestGetWebhookAnnotationsWithCertManager(t *testing.T) {
	infrastructure.InitializeForTestingWithCertManager(infrastructure.Kubernetes)
	annotations := getWebhookAnnotations("test-namespace")
	assert.Equal(t, map[string]string{
		"cert-manager.io/inject-ca-from": "test-namespace/devworkspace-controller-serving-cert",
	}, annotations)
}

func TestGetWebhookAnnotationsWithOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	annotations := getWebhookAnnotations("test-namespace")
	assert.Equal(t, map[string]string{
		"service.beta.openshift.io/inject-cabundle": "true",
	}, annotations)
}

func TestGetWebhookAnnotationsWithKubernetes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	annotations := getWebhookAnnotations("test-namespace")
	assert.Empty(t, annotations)
}

func TestGetWebhookAnnotationsWithCertManagerOnOpenShift(t *testing.T) {
	infrastructure.InitializeForTestingWithCertManager(infrastructure.OpenShiftv4)
	annotations := getWebhookAnnotations("test-namespace")
	assert.Equal(t, map[string]string{
		"cert-manager.io/inject-ca-from": "test-namespace/devworkspace-controller-serving-cert",
	}, annotations)
}
