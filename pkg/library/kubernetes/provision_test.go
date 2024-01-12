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

package kubernetes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

type testCase struct {
	Name             string     `json:"name,omitempty"`
	Input            testInput  `json:"input,omitempty"`
	Output           testOutput `json:"output,omitempty"`
	originalFilename string
}

type testInput struct {
	Components      []dw.Component `json:"components,omitempty"`
	ExistingObjects clusterObjects `json:"existingObjects,omitempty"`
}

type testOutput struct {
	ExpectedObjects clusterObjects `json:"expectedObjects,omitempty"`
	ErrRegexp       *string        `json:"errRegexp,omitempty"`
}

type clusterObjects struct {
	Pods     []corev1.Pod     `json:"pods,omitempty"`
	Services []corev1.Service `json:"services,omitempty"`
}

const (
	testID               = "test-devworkspaceID"
	testCreatorID        = "test-creatorID"
	testDevWorkspaceName = "test-devworkspace"
	testDevWorkspaceUID  = "test-UID"
	testNamespace        = "test-devworkspace"
)

var testDevWorkspace = &dw.DevWorkspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testDevWorkspaceName,
		Namespace: testNamespace,
		Labels: map[string]string{
			constants.DevWorkspaceCreatorLabel: testCreatorID,
		},
		UID: testDevWorkspaceUID,
	},
	Spec: dw.DevWorkspaceSpec{
		Template: dw.DevWorkspaceTemplateSpec{
			DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{},
		},
	},
	Status: dw.DevWorkspaceStatus{
		DevWorkspaceId: testID,
	},
}

func TestHandleKubernetesComponents(t *testing.T) {
	if err := InitializeDeserializer(testScheme); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer func() { decoder = nil }()
	tests := loadAllTestCasesOrPanic(t, "testdata/provision_tests")
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (%s)", tt.Name, tt.originalFilename), func(t *testing.T) {
			testClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(collectClusterObj(tt.Input.ExistingObjects)...).Build()
			api := sync.ClusterAPI{
				Client: testClient,
				Scheme: testScheme,
				Logger: testr.New(t),
			}
			wksp := &common.DevWorkspaceWithConfig{
				DevWorkspace: testDevWorkspace.DeepCopy(),
			}
			wksp.Spec.Template.Components = append(wksp.Spec.Template.Components, tt.Input.Components...)
			// Repeat function as long as it returns RetryError
			i := 0
			maxIters := 30
			var err error
			retryErr := &dwerrors.RetryError{}
			for err = HandleKubernetesComponents(wksp, api); errors.As(err, &retryErr); err = HandleKubernetesComponents(wksp, api) {
				i += 1
				assert.LessOrEqual(t, i, maxIters, "HandleKubernetesComponents did no complete within %d iterations", maxIters)
			}

			if tt.Output.ErrRegexp != nil {
				if !assert.Error(t, err, "Expected error to be returned") {
					return
				}
				assert.Regexp(t, *tt.Output.ErrRegexp, err.Error())
			} else {
				if !assert.NoError(t, err, "Unexpected error returned") {
					return
				}
				for _, obj := range collectClusterObj(tt.Output.ExpectedObjects) {
					objType := reflect.TypeOf(obj).Elem()
					clusterObj := reflect.New(objType).Interface().(client.Object)
					err := testClient.Get(api.Ctx, types.NamespacedName{Name: obj.GetName(), Namespace: wksp.Namespace}, clusterObj)
					if !assert.NoError(t, err, "Expect object to be created on cluster: %s %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()) {
						return
					}
					assert.True(t, cmp.Equal(obj, clusterObj, cmpopts.IgnoreTypes(metav1.ObjectMeta{}, metav1.TypeMeta{})),
						"Expect objects to match: %s", cmp.Diff(obj, clusterObj, cmpopts.IgnoreTypes(metav1.ObjectMeta{}, metav1.TypeMeta{})))
					assert.Equal(t, clusterObj.GetLabels()[constants.DevWorkspaceIDLabel], testID, "Object should get devworkspace ID label")
					assert.Equal(t, clusterObj.GetLabels()[constants.DevWorkspaceCreatorLabel], testCreatorID, "Object should get devworkspace ID label")
					assert.Contains(t, clusterObj.GetOwnerReferences(), metav1.OwnerReference{
						Kind:       "DevWorkspace",
						APIVersion: "workspace.devfile.io/v1alpha2",
						Name:       testDevWorkspaceName,
						UID:        testDevWorkspaceUID,
					})
				}
			}
		})
	}
}

func TestSecretAndConfigMapProvisioning(t *testing.T) {
	if err := InitializeDeserializer(testScheme); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer func() { decoder = nil }()

	testClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	api := sync.ClusterAPI{
		Client: testClient,
		Scheme: testScheme,
		Logger: testr.New(t),
	}
	wksp := &common.DevWorkspaceWithConfig{
		DevWorkspace: testDevWorkspace.DeepCopy(),
	}
	cmInline := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-configmap"},"data":{"test":"data"}}`
	secretInline := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"test-secret"},"data":{"test":"dGVzdAo="}}`
	wksp.Spec.Template.Components = append(wksp.Spec.Template.Components,
		dw.Component{
			Name: "test-configmap",
			ComponentUnion: dw.ComponentUnion{
				Kubernetes: &dw.KubernetesComponent{
					K8sLikeComponent: dw.K8sLikeComponent{
						DeployByDefault: pointer.BoolPtr(true),
						K8sLikeComponentLocation: dw.K8sLikeComponentLocation{
							Inlined: cmInline,
						},
					},
				},
			},
		},
		dw.Component{
			Name: "test-secret",
			ComponentUnion: dw.ComponentUnion{
				Kubernetes: &dw.KubernetesComponent{
					K8sLikeComponent: dw.K8sLikeComponent{
						DeployByDefault: pointer.BoolPtr(true),
						K8sLikeComponentLocation: dw.K8sLikeComponentLocation{
							Inlined: secretInline,
						},
					},
				},
			},
		},
	)
	// Repeat function as long as it returns RetryError
	i := 0
	maxIters := 30
	var err error
	retryErr := &dwerrors.RetryError{}
	for err = HandleKubernetesComponents(wksp, api); errors.As(err, &retryErr); err = HandleKubernetesComponents(wksp, api) {
		i += 1
		assert.LessOrEqual(t, i, maxIters, "HandleKubernetesComponents did no complete within %d iterations", maxIters)
	}
	if !assert.NoError(t, err) {
		return
	}

	clusterConfigmap := &corev1.ConfigMap{}
	err = testClient.Get(api.Ctx, types.NamespacedName{Name: "test-configmap", Namespace: wksp.Namespace}, clusterConfigmap)
	if !assert.NoError(t, err, "Expect configmap 'test-configmap' to be created on cluster") {
		return
	}
	assert.Contains(t, clusterConfigmap.GetLabels(), constants.DevWorkspaceWatchConfigMapLabel)

	clusterSecret := &corev1.Secret{}
	err = testClient.Get(api.Ctx, types.NamespacedName{Name: "test-secret", Namespace: wksp.Namespace}, clusterSecret)
	if !assert.NoError(t, err, "Expect secret 'test-secret' to be created on cluster") {
		return
	}
	assert.Contains(t, clusterSecret.GetLabels(), constants.DevWorkspaceWatchSecretLabel)
}

func TestHasKubelikeComponent(t *testing.T) {
	noComponents := loadTestCaseOrPanic(t, "testdata/provision_tests/no-k8s-components-devworkspace.yaml")
	workspaceWithoutK8sComponents := &common.DevWorkspaceWithConfig{
		DevWorkspace: testDevWorkspace.DeepCopy(),
	}
	workspaceWithoutK8sComponents.Spec.Template.Components = append(workspaceWithoutK8sComponents.Spec.Template.Components, noComponents.Input.Components...)
	assert.False(t, HasKubelikeComponent(workspaceWithoutK8sComponents))

	hasComponents := loadTestCaseOrPanic(t, "testdata/provision_tests/creates-k8s-objects.yaml")
	workspaceWithK8sComponents := &common.DevWorkspaceWithConfig{
		DevWorkspace: testDevWorkspace.DeepCopy(),
	}
	workspaceWithK8sComponents.Spec.Template.Components = append(workspaceWithK8sComponents.Spec.Template.Components, hasComponents.Input.Components...)
	assert.True(t, HasKubelikeComponent(workspaceWithK8sComponents))
}

func loadAllTestCasesOrPanic(t *testing.T, fromDir string) []testCase {
	files, err := os.ReadDir(fromDir)
	if err != nil {
		t.Fatal(err)
	}
	var tests []testCase
	for _, file := range files {
		if file.IsDir() {
			tests = append(tests, loadAllTestCasesOrPanic(t, filepath.Join(fromDir, file.Name()))...)
		} else {
			tests = append(tests, loadTestCaseOrPanic(t, filepath.Join(fromDir, file.Name())))
		}
	}
	return tests
}

func loadTestCaseOrPanic(t *testing.T, testPath string) testCase {
	bytes, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	var test testCase
	if err := yaml.Unmarshal(bytes, &test); err != nil {
		t.Fatal(err)
	}
	test.originalFilename = testPath
	return test
}

func collectClusterObj(clusterObjs clusterObjects) []client.Object {
	var objs []client.Object
	for _, pod := range clusterObjs.Pods {
		pod := pod
		pod.Namespace = testNamespace
		objs = append(objs, &pod)
	}
	for _, svc := range clusterObjs.Services {
		svc := svc
		svc.Namespace = testNamespace
		objs = append(objs, &svc)
	}
	return objs
}
