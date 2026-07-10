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

package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCertManagerDetectedReturnsTrueWhenCertManagerInstalled(t *testing.T) {
	InitializeForTestingWithCertManager(Kubernetes)
	assert.True(t, CertManagerDetected())
}

func TestCertManagerDetectedReturnsFalseWhenCertManagerNotInstalled(t *testing.T) {
	InitializeForTesting(Kubernetes)
	assert.False(t, CertManagerDetected())
}

func TestCertManagerDetectedWithOpenShift(t *testing.T) {
	InitializeForTestingWithCertManager(OpenShiftv4)
	assert.True(t, CertManagerDetected())
}

func TestCertManagerDetectedPanicsWhenNotInitialized(t *testing.T) {
	initialized = false
	defer func() {
		InitializeForTesting(Kubernetes)
	}()
	assert.Panics(t, func() {
		CertManagerDetected()
	})
}

func TestInitializeForTestingSetsCertManagerDetectedToFalse(t *testing.T) {
	InitializeForTesting(OpenShiftv4)
	assert.False(t, CertManagerDetected())
	assert.True(t, IsOpenShift())
}

func TestInitializeForTestingWithCertManagerSetsDetectedToTrue(t *testing.T) {
	InitializeForTestingWithCertManager(Kubernetes)
	assert.True(t, CertManagerDetected())
}
