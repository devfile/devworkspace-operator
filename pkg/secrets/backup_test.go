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

package secrets_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/secrets"
)

func TestSecrets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Secrets Suite")
}

// buildScheme returns a minimal runtime.Scheme for tests: core v1 and the DWO API types.
func buildScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(dwv2.AddToScheme(scheme)).To(Succeed())
	Expect(controllerv1alpha1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

// makeWorkspace returns a minimal DevWorkspace in the given namespace.
func makeWorkspace(namespace string) *dwv2.DevWorkspace {
	return &dwv2.DevWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workspace",
			Namespace: namespace,
		},
	}
}

// makeConfig returns an OperatorConfiguration with BackupCronJob configured to use the given auth secret name.
func makeConfig(authSecretName string) *controllerv1alpha1.OperatorConfiguration {
	return &controllerv1alpha1.OperatorConfiguration{
		Workspace: &controllerv1alpha1.WorkspaceConfig{
			BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
				Registry: &controllerv1alpha1.RegistryConfig{
					Path:       "example.registry.io/org",
					AuthSecret: authSecretName,
				},
			},
		},
	}
}

// makeSecret returns a corev1.Secret with the given name and namespace.
func makeSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{"auth": []byte("dXNlcjpwYXNz")},
		Type: corev1.SecretTypeDockerConfigJson,
	}
}

var _ = Describe("HandleRegistryAuthSecret (restore path: operatorConfigNamespace='')", func() {
	const workspaceNS = "user-namespace"

	var (
		ctx    context.Context
		scheme *runtime.Scheme
		log    = zap.New(zap.UseDevMode(true)).WithName("SecretsTest")
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = buildScheme()
	})

	It("returns the secret directly when the configured name is present in the workspace namespace", func() {
		By("creating a workspace-namespace secret whose name matches the configured auth secret name")
		configuredName := "quay-backup-auth"
		secret := makeSecret(configuredName, workspaceNS)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig(configuredName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(configuredName))
	})

	It("returns the canonical backup auth secret as fallback when configured name is absent (the bug fix case)", func() {
		By("creating only the canonical DevWorkspaceBackupAuthSecretName secret in the workspace namespace")
		canonicalSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, workspaceNS)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(canonicalSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		// Configured name is something like "quay-backup-auth" — different from the canonical name.
		config := makeConfig("quay-backup-auth")

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil(), "should fall back to the canonical secret copied by CopySecret")
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
	})

	It("returns nil, nil when neither the configured name nor the canonical name exists in the workspace namespace", func() {
		By("using a fake client with no secrets at all")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig("quay-backup-auth")

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("returns the secret on the first lookup when the configured name IS the canonical name (no duplicate query)", func() {
		By("creating the canonical secret in the workspace namespace and configuring the same name")
		canonicalSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, workspaceNS)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(canonicalSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		// Configured name equals the canonical constant — must be found on the first Get.
		config := makeConfig(constants.DevWorkspaceBackupAuthSecretName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
	})

	It("propagates a real (non-NotFound) error from the fallback lookup", func() {
		By("wrapping a fake client so that the fallback lookup returns a server error")
		// The configured name differs from the canonical name, so the code will attempt the fallback
		// lookup. We simulate that lookup returning a non-NotFound error.
		errClient := &errorOnNameClient{
			Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
			failName: constants.DevWorkspaceBackupAuthSecretName,
			failErr:  k8sErrors.NewInternalError(errors.New("simulated etcd timeout")),
		}
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig("quay-backup-auth")

		result, err := secrets.HandleRegistryAuthSecret(ctx, errClient, workspace, config, "", scheme, log)
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("simulated etcd timeout"))
	})
})

// errorOnNameClient is a thin client wrapper that injects an error for a specific secret name.
type errorOnNameClient struct {
	client.Client
	failName string
	failErr  error
}

func (e *errorOnNameClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if secret, ok := obj.(*corev1.Secret); ok {
		_ = secret
		if key.Name == e.failName {
			return e.failErr
		}
	}
	return e.Client.Get(ctx, key, obj, opts...)
}

// Ensure errorOnNameClient satisfies client.Client at compile time.
var _ client.Client = &errorOnNameClient{}

// secretGR is the GroupResource for secrets, used when constructing test errors.
var secretGR = schema.GroupResource{Group: "", Resource: "secrets"} //nolint:unused
