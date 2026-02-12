//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
)

// createTestDevWorkspace creates a test DevWorkspace with common test values
func createTestDevWorkspace() *dwv2.DevWorkspace {
	return &dwv2.DevWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workspace",
			Namespace: "test-namespace",
		},
		Status: dwv2.DevWorkspaceStatus{
			DevWorkspaceId: "test-workspace-id",
		},
	}
}

var _ = Describe("ensureJobRunnerRBAC OpenShift functionality", func() {
	var (
		ctx                     context.Context
		fakeClient              client.Client
		backupCronJobReconciler BackupCronJobReconciler
		log                     logr.Logger
		workspace               *dwv2.DevWorkspace
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = zap.New(zap.UseDevMode(true)).WithName("RBACTest")

		workspace = createTestDevWorkspace()

		scheme := runtime.NewScheme()
		Expect(dwv2.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(rbacv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		backupCronJobReconciler = BackupCronJobReconciler{
			Client: fakeClient,
			Log:    log,
			Scheme: scheme,
		}
	})

	Context("On OpenShift platform", func() {
		BeforeEach(func() {
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
		})

		It("should create OpenShift-specific resources (RoleBinding and ImageStream)", func() {
			Expect(fakeClient.Create(ctx, workspace)).To(Succeed())

			err := backupCronJobReconciler.ensureJobRunnerRBAC(ctx, workspace)
			Expect(err).ToNot(HaveOccurred())

			// Verify RoleBinding for image-builder was created
			roleBinding := &rbacv1.RoleBinding{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "devworkspace-image-builder-test-workspace-id",
				Namespace: workspace.Namespace,
			}, roleBinding)
			Expect(err).ToNot(HaveOccurred())
			Expect(roleBinding.Labels).To(HaveKeyWithValue("controller.devfile.io/devworkspace_id", "test-workspace-id"))
			Expect(roleBinding.Subjects).To(HaveLen(1))
			Expect(roleBinding.Subjects[0].Name).To(Equal("devworkspace-job-runner-test-workspace-id"))
			Expect(roleBinding.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(roleBinding.RoleRef.Name).To(Equal("system:image-builder"))

			// Verify ImageStream was created
			imageStream := &unstructured.Unstructured{}
			imageStream.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "image.openshift.io",
				Version: "v1",
				Kind:    "ImageStream",
			})
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      workspace.Name,
				Namespace: workspace.Namespace,
			}, imageStream)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageStream.GetLabels()).To(HaveKeyWithValue("controller.devfile.io/devworkspace_id", "test-workspace-id"))

			// Verify ImageStream spec
			spec, found, err := unstructured.NestedMap(imageStream.Object, "spec")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			lookupPolicy, found, err := unstructured.NestedMap(spec, "lookupPolicy")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			local, found, err := unstructured.NestedBool(lookupPolicy, "local")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(local).To(BeTrue())
		})
	})
})

var _ = Describe("ensureImagePushRoleBinding", func() {
	var (
		ctx                     context.Context
		fakeClient              client.Client
		backupCronJobReconciler BackupCronJobReconciler
		log                     logr.Logger
		workspace               *dwv2.DevWorkspace
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = zap.New(zap.UseDevMode(true)).WithName("ImagePushRoleBindingTest")
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		workspace = createTestDevWorkspace()

		scheme := runtime.NewScheme()
		Expect(dwv2.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(rbacv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		backupCronJobReconciler = BackupCronJobReconciler{
			Client: fakeClient,
			Log:    log,
			Scheme: scheme,
		}
	})

	It("should create RoleBinding with correct properties", func() {
		clusterAPI := sync.ClusterAPI{
			Client: fakeClient,
			Scheme: backupCronJobReconciler.Scheme,
			Logger: log,
			Ctx:    ctx,
		}
		saName := "test-service-account"

		err := backupCronJobReconciler.ensureImagePushRoleBinding(saName, workspace, clusterAPI)
		Expect(err).ToNot(HaveOccurred())

		roleBinding := &rbacv1.RoleBinding{}
		err = fakeClient.Get(ctx, types.NamespacedName{
			Name:      "devworkspace-image-builder-test-workspace-id",
			Namespace: workspace.Namespace,
		}, roleBinding)
		Expect(err).ToNot(HaveOccurred())

		// Verify labels
		Expect(roleBinding.Labels).To(HaveKeyWithValue("controller.devfile.io/devworkspace_id", "test-workspace-id"))

		// Verify subjects
		Expect(roleBinding.Subjects).To(HaveLen(1))
		Expect(roleBinding.Subjects[0].Kind).To(Equal(rbacv1.ServiceAccountKind))
		Expect(roleBinding.Subjects[0].Name).To(Equal(saName))
		Expect(roleBinding.Subjects[0].Namespace).To(Equal(workspace.Namespace))

		// Verify role reference
		Expect(roleBinding.RoleRef.Kind).To(Equal("ClusterRole"))
		Expect(roleBinding.RoleRef.Name).To(Equal("system:image-builder"))
		Expect(roleBinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
	})
})

var _ = Describe("ensureImageStreamForBackup", func() {
	var (
		ctx                     context.Context
		fakeClient              client.Client
		backupCronJobReconciler BackupCronJobReconciler
		log                     logr.Logger
		workspace               *dwv2.DevWorkspace
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = zap.New(zap.UseDevMode(true)).WithName("ImageStreamTest")
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		workspace = createTestDevWorkspace()

		scheme := runtime.NewScheme()
		Expect(dwv2.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		backupCronJobReconciler = BackupCronJobReconciler{
			Client: fakeClient,
			Log:    log,
			Scheme: scheme,
		}
	})

	It("should create ImageStream with correct properties", func() {
		clusterAPI := sync.ClusterAPI{
			Client: fakeClient,
			Scheme: backupCronJobReconciler.Scheme,
			Logger: log,
			Ctx:    ctx,
		}

		err := backupCronJobReconciler.ensureImageStreamForBackup(ctx, workspace, clusterAPI)
		Expect(err).ToNot(HaveOccurred())

		imageStream := &unstructured.Unstructured{}
		imageStream.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "image.openshift.io",
			Version: "v1",
			Kind:    "ImageStream",
		})
		err = fakeClient.Get(ctx, types.NamespacedName{
			Name:      workspace.Name,
			Namespace: workspace.Namespace,
		}, imageStream)
		Expect(err).ToNot(HaveOccurred())

		// Verify metadata
		Expect(imageStream.GetName()).To(Equal(workspace.Name))
		Expect(imageStream.GetNamespace()).To(Equal(workspace.Namespace))
		Expect(imageStream.GetLabels()).To(HaveKeyWithValue("controller.devfile.io/devworkspace_id", "test-workspace-id"))

		// Verify GVK
		Expect(imageStream.GetAPIVersion()).To(Equal("image.openshift.io/v1"))
		Expect(imageStream.GetKind()).To(Equal("ImageStream"))

		// Verify spec
		spec, found, err := unstructured.NestedMap(imageStream.Object, "spec")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		lookupPolicy, found, err := unstructured.NestedMap(spec, "lookupPolicy")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		local, found, err := unstructured.NestedBool(lookupPolicy, "local")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(local).To(BeTrue())
	})
})
