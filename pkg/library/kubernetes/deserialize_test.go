// Copyright (c) 2019-2022 Red Hat, Inc.
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

package kubernetes

import (
	"fmt"
	"os"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	scheme = runtime.NewScheme()
	api    = sync.ClusterAPI{
		Scheme: scheme,
	}
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
}

func TestDeserializeObject(t *testing.T) {
	InitializeDeserializer(scheme)
	defer func() {
		decoder = nil
	}()
	tests := []struct {
		name              string
		filePath          string
		expectedObjStub   client.Object
		expectedErrRegexp string
	}{
		{
			name:            "Deserializes Pod",
			filePath:        "testdata/k8s_objects/pod.yaml",
			expectedObjStub: &corev1.Pod{},
		},
		{
			name:            "Deserializes Service",
			filePath:        "testdata/k8s_objects/service.yaml",
			expectedObjStub: &corev1.Service{},
		},
		{
			name:            "Deserializes ConfigMap",
			filePath:        "testdata/k8s_objects/configmap.yaml",
			expectedObjStub: &corev1.ConfigMap{},
		},
		{
			name:              "Kubernetes list",
			filePath:          "testdata/k8s_objects/kubernetes-list.yaml",
			expectedErrRegexp: "objects of kind 'List' are unsupported",
		},
		{
			name:              "Random yaml that is not a Kubernetes object",
			filePath:          "testdata/k8s_objects/non-k8s-object.yaml",
			expectedErrRegexp: "Object 'Kind' is missing",
		},
		{
			name:              "Object with unrecognized kind",
			filePath:          "testdata/k8s_objects/unrecognized-kind.yaml",
			expectedErrRegexp: "no kind .* is registered for version .* in scheme",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.name, tt.filePath), func(t *testing.T) {
			jsonBytes := readBytesFromFile(t, tt.filePath)
			actualObj, err := deserializeToObject(jsonBytes, api)
			if tt.expectedErrRegexp != "" {
				if !assert.Error(t, err, "Expect error to be returned") {
					return
				}
				assert.Regexp(t, tt.expectedErrRegexp, err.Error(), "Expect error to match pattern")
			} else {
				if !assert.NoError(t, err, "Expect no error to be returned") {
					return
				}
				err := yaml.Unmarshal(jsonBytes, tt.expectedObjStub)
				assert.NoError(t, err)
				assert.True(t, cmp.Equal(tt.expectedObjStub, actualObj), cmp.Diff(tt.expectedObjStub, actualObj))
			}
		})
	}
}

func TestErrorIfDeserializerNotInitialized(t *testing.T) {
	_, err := deserializeToObject([]byte(""), api)
	assert.Error(t, err)
	assert.Equal(t, "kubernetes object deserializer is not initialized", err.Error())
}

func readBytesFromFile(t *testing.T, filePath string) []byte {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}
