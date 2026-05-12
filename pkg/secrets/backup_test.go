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

	It("returns the predefined secret when it exists in the workspace namespace", func() {
		By("creating the predefined DevWorkspaceBackupAuthSecretName secret in the workspace namespace")
		predefinedSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, workspaceNS)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(predefinedSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig("quay-backup-auth")

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
	})

	It("returns nil when the predefined secret does not exist in the workspace namespace", func() {
		By("using a fake client with no secrets")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig("quay-backup-auth")

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, "", scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("propagates a non-NotFound error from the workspace namespace lookup", func() {
		By("wrapping a fake client so that the predefined name lookup returns a server error")
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

var _ = Describe("HandleRegistryAuthSecret (backup path: operatorConfigNamespace set)", func() {
	const (
		workspaceNS = "user-namespace"
		operatorNS  = "devworkspace-controller"
	)

	var (
		ctx    context.Context
		scheme *runtime.Scheme
		log    = zap.New(zap.UseDevMode(true)).WithName("SecretsTest")
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = buildScheme()
	})

	It("returns the secret from workspace namespace if it exists", func() {
		By("creating a secret in the workspace namespace")
		workspaceSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, workspaceNS)
		workspaceSecret.Data = map[string][]byte{"auth": []byte("user-credentials")}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(workspaceSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig(constants.DevWorkspaceBackupAuthSecretName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, operatorNS, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
		Expect(result.Namespace).To(Equal(workspaceNS))
		Expect(result.Data["auth"]).To(Equal([]byte("user-credentials")))
	})

	It("returns nil when AuthSecret is not configured and secret not found in workspace namespace (anonymous registry access)", func() {
		By("using a config with empty AuthSecret")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig("") // Empty AuthSecret

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, operatorNS, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("copies secret from operator namespace when AuthSecret is configured and secret not found in workspace namespace", func() {
		By("creating a secret in the operator namespace")
		operatorSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, operatorNS)
		operatorSecret.Data = map[string][]byte{"auth": []byte("operator-credentials")}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(operatorSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig(constants.DevWorkspaceBackupAuthSecretName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, operatorNS, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
		Expect(result.Namespace).To(Equal(workspaceNS))
		Expect(result.Data["auth"]).To(Equal([]byte("operator-credentials")))

		By("verifying the secret was created in the workspace namespace")
		copiedSecret := &corev1.Secret{}
		err = fakeClient.Get(ctx, client.ObjectKey{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspaceNS,
		}, copiedSecret)
		Expect(err).NotTo(HaveOccurred())
		Expect(copiedSecret.Data["auth"]).To(Equal([]byte("operator-credentials")))

		By("verifying the copied secret has the watch-secret label")
		Expect(copiedSecret.Labels).To(HaveKeyWithValue(constants.DevWorkspaceWatchSecretLabel, "true"))

		By("verifying the copied secret has an owner reference to the workspace")
		Expect(copiedSecret.OwnerReferences).To(HaveLen(1))
		Expect(copiedSecret.OwnerReferences[0].Name).To(Equal(workspace.Name))
		Expect(copiedSecret.OwnerReferences[0].Kind).To(Equal("DevWorkspace"))
		Expect(copiedSecret.OwnerReferences[0].Controller).NotTo(BeNil())
		Expect(*copiedSecret.OwnerReferences[0].Controller).To(BeTrue())
	})

	It("NEVER overwrites user-provided secret even if operator has different credentials", func() {
		By("creating different secrets in both namespaces")
		userSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, workspaceNS)
		userSecret.Data = map[string][]byte{"auth": []byte("user-scoped-credentials")}

		operatorSecret := makeSecret(constants.DevWorkspaceBackupAuthSecretName, operatorNS)
		operatorSecret.Data = map[string][]byte{"auth": []byte("operator-wide-credentials")}

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(userSecret, operatorSecret).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig(constants.DevWorkspaceBackupAuthSecretName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, operatorNS, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())

		By("verifying the user's secret was NOT overwritten")
		Expect(result.Data["auth"]).To(Equal([]byte("user-scoped-credentials")), "User's secret should be preserved")

		By("verifying the secret in workspace namespace still has user's credentials")
		workspaceSecret := &corev1.Secret{}
		err = fakeClient.Get(ctx, client.ObjectKey{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspaceNS,
		}, workspaceSecret)
		Expect(err).NotTo(HaveOccurred())
		Expect(workspaceSecret.Data["auth"]).To(Equal([]byte("user-scoped-credentials")), "User's secret must never be overwritten")
	})

	It("returns error when AuthSecret is configured but secret not found in operator namespace", func() {
		By("using a config with AuthSecret but no secret in operator namespace")
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		workspace := makeWorkspace(workspaceNS)
		config := makeConfig(constants.DevWorkspaceBackupAuthSecretName)

		result, err := secrets.HandleRegistryAuthSecret(ctx, fakeClient, workspace, config, operatorNS, scheme, log)
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
		Expect(k8sErrors.IsNotFound(err)).To(BeTrue(), "Should return a NotFound error when secret doesn't exist in operator namespace")
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

var _ = Describe("CopySecret", func() {
	const (
		workspaceNS = "user-namespace"
		operatorNS  = "operator-namespace"
	)

	var (
		ctx    context.Context
		scheme *runtime.Scheme
		log    = zap.New(zap.UseDevMode(true)).WithName("SecretsTest")
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = buildScheme()
	})

	It("creates the secret without ownerReferences", func() {
		By("copying a source secret into the workspace namespace")
		sourceSecret := makeSecret("quay-push-secret", operatorNS)
		workspace := makeWorkspace(workspaceNS)

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		result, err := secrets.CopySecret(ctx, fakeClient, workspace, sourceSecret, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())
		Expect(result.Name).To(Equal(constants.DevWorkspaceBackupAuthSecretName))
		Expect(result.Namespace).To(Equal(workspaceNS))

		By("verifying the created secret has no ownerReferences")
		created := &corev1.Secret{}
		err = fakeClient.Get(ctx, client.ObjectKey{
			Name:      constants.DevWorkspaceBackupAuthSecretName,
			Namespace: workspaceNS,
		}, created)
		Expect(err).NotTo(HaveOccurred())
		Expect(created.OwnerReferences).To(BeEmpty())
	})

	It("preserves the secret data and type from the source", func() {
		sourceSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "quay-push-secret",
				Namespace: operatorNS,
			},
			Data: map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
			Type: corev1.SecretTypeDockerConfigJson,
		}
		workspace := makeWorkspace(workspaceNS)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		result, err := secrets.CopySecret(ctx, fakeClient, workspace, sourceSecret, scheme, log)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey(".dockerconfigjson"))
		Expect(result.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
	})
})
