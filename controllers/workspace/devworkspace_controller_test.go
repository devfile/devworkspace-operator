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

package controllers_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	workspacecontroller "github.com/devfile/devworkspace-operator/controllers/workspace"
	"github.com/devfile/devworkspace-operator/controllers/workspace/internal/testutil"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/pkg/library/restore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func loadObjectFromFile(objName string, obj client.Object, filename string) error {
	path := filepath.Join("testdata", filename)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, obj)
	if err != nil {
		return err
	}
	obj.SetNamespace(testNamespace)
	obj.SetName(objName)

	return nil
}

var _ = Describe("DevWorkspace Controller", func() {
	Context("Basic DevWorkspace Tests", func() {
		It("Sets DevWorkspace ID and Starting status", func() {
			By("Reading DevWorkspace from testdata file")
			devworkspace := &dw.DevWorkspace{}
			Expect(loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")).Should(Succeed())

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := namespacedName(devWorkspaceName, testNamespace)
			defer deleteDevWorkspace(devWorkspaceName)

			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwNamespacedName, createdDW)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspace should exist in cluster")

			By("Checking DevWorkspace ID has been set")
			Eventually(func() (devworkspaceID string, err error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.DevWorkspaceId, nil
			}, timeout, interval).Should(Not(Equal("")), "Should set DevWorkspace ID after creation")

			By("Checking DevWorkspace Status is updated to starting")
			Eventually(func() (phase dw.DevWorkspacePhase, err error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusStarting), "DevWorkspace should have Starting phase")
			Expect(createdDW.Status.Message).ShouldNot(BeEmpty(), "Status message should be set for starting workspaces")
			startingCondition := conditions.GetConditionByType(createdDW.Status.Conditions, conditions.Started)
			Expect(startingCondition).ShouldNot(BeNil(), "Should have 'Starting' condition")
			Expect(startingCondition.Status).Should(Equal(corev1.ConditionTrue), "Starting condition should be 'true'")
		})

		It("Allows overriding the DevWorkspace ID", func() {
			By("Reading DevWorkspace from testdata file")
			devworkspace := &dw.DevWorkspace{}
			Expect(loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")).Should(Succeed())

			if devworkspace.Annotations == nil {
				devworkspace.Annotations = map[string]string{}
			}
			devworkspace.Annotations[constants.WorkspaceIdOverrideAnnotation] = "test-workspace-id"

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := namespacedName(devWorkspaceName, testNamespace)
			defer deleteDevWorkspace(devWorkspaceName)

			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwNamespacedName, createdDW)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspace should exist in cluster")

			By("Checking DevWorkspace ID has been set")
			Eventually(func() (devworkspaceID string, err error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.DevWorkspaceId, nil
			}, timeout, interval).Should(Not(Equal("")), "Should set DevWorkspace ID after creation")
			Expect(createdDW.Status.DevWorkspaceId).Should(Equal("test-workspace-id"), "DevWorkspace ID should be set from override annotation")
		})

		It("Forbids duplicate workspace IDs from override", func() {
			By("Reading DevWorkspace from testdata file")
			devworkspace := &dw.DevWorkspace{}
			Expect(loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")).Should(Succeed())

			if devworkspace.Annotations == nil {
				devworkspace.Annotations = map[string]string{}
			}
			devworkspace.Annotations[constants.WorkspaceIdOverrideAnnotation] = "test-workspace-id"

			devworkspace2 := devworkspace.DeepCopy()
			devworkspace2.Name = fmt.Sprintf("%s-dupe", devworkspace2.Name)

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := namespacedName(devWorkspaceName, testNamespace)
			defer deleteDevWorkspace(devWorkspaceName)

			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwNamespacedName, createdDW)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspace should exist in cluster")

			By("Waiting for the first DevWorkspace to have its ID set")
			Eventually(func() (string, error) {
				if err := k8sClient.Get(ctx, dwNamespacedName, createdDW); err != nil {
					return "", err
				}
				return createdDW.Status.DevWorkspaceId, nil
			}, timeout, interval).Should(Equal("test-workspace-id"), "First DevWorkspace should have ID set from override annotation")

			By("Creating a DevWorkspace that duplicates the workspace ID of the first")
			Expect(k8sClient.Create(ctx, devworkspace2)).Should(Succeed())
			defer deleteDevWorkspace(devworkspace2.Name)

			By("Checking that duplicate DevWorkspace enters failed Phase")
			createdDW2 := &dw.DevWorkspace{}
			Eventually(func() (phase dw.DevWorkspacePhase, err error) {
				if err := k8sClient.Get(ctx, namespacedName(devworkspace2.Name, testNamespace), createdDW2); err != nil {
					return "", err
				}
				return createdDW2.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusFailed), "DevWorkspace with duplicate ID should fail to start")
		})
	})

	Context("Workspace Objects creation", func() {

		BeforeEach(func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Creates roles and rolebindings", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			By("Checking that common role is created")
			dwRole := &rbacv1.Role{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, namespacedName(common.WorkspaceRoleName(), testNamespace), dwRole); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common Role should be created for DevWorkspace")

			By("Checking that common rolebinding is created")
			dwRoleBinding := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, namespacedName(common.WorkspaceRolebindingName(), testNamespace), dwRoleBinding); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common RoleBinding should be created for DevWorkspace")
			Expect(dwRoleBinding.RoleRef.Name).Should(Equal(dwRole.Name), "Rolebinding should refer to DevWorkspace role")
			expectedSubject := rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      common.ServiceAccountName(&common.DevWorkspaceWithConfig{DevWorkspace: devworkspace, Config: testControllerCfg}),
				Namespace: testNamespace,
			}
			Expect(dwRoleBinding.Subjects).Should(ContainElement(expectedSubject), "Rolebinding should bind to serviceaccounts in current namespace")
		})

		It("Creates DevWorkspaceRouting", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Checking that DevWorkspaceRouting is created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(dwrName, testNamespace), dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting is present on cluster")

			Expect(string(dwr.Spec.RoutingClass)).Should(Equal(devworkspace.Spec.RoutingClass), "RoutingClass should be propagated to DevWorkspaceRouting")
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(dwr.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Routing should be owned by DevWorkspace")
			Expect(dwr.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs Routing mainURL to DevWorkspace", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			Eventually(func() (string, error) {
				if err := k8sClient.Get(ctx, namespacedName(devWorkspaceName, testNamespace), devworkspace); err != nil {
					return "", err
				}
				return devworkspace.Status.MainUrl, nil
			}, timeout, interval).Should(Equal("test-url"))

		})

		It("Creates workspace metadata configmap", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			metadataCM := &corev1.ConfigMap{}
			Eventually(func() error {
				cmNN := namespacedName(common.MetadataConfigMapName(workspaceID), testNamespace)
				return k8sClient.Get(ctx, cmNN, metadataCM)
			}, timeout, interval).Should(Succeed(), "Should create workspace metadata configmap")

			// Check that metadata configmap is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(metadataCM.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Metadata configmap should be owned by DevWorkspace")
			Expect(metadataCM.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")

			originalDevfileYaml, originalPresent := metadataCM.Data["original.devworkspace.yaml"]
			Expect(originalPresent).Should(BeTrue(), "Metadata configmap should contain original devfile spec")
			originalDevfile := &dw.DevWorkspaceTemplateSpec{}
			Expect(yaml.Unmarshal([]byte(originalDevfileYaml), originalDevfile)).Should(Succeed())
			Expect(originalDevfile).Should(Equal(&devworkspace.Spec.Template))
			_, flattenedPresent := metadataCM.Data["flattened.devworkspace.yaml"]
			Expect(flattenedPresent).Should(BeTrue(), "Metadata configmap should contain flattened devfile spec")
		})

		It("Syncs the DevWorkspace ServiceAccount", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				saNN := namespacedName(common.ServiceAccountName(&common.DevWorkspaceWithConfig{DevWorkspace: devworkspace, Config: testControllerCfg}), testNamespace)
				return k8sClient.Get(ctx, saNN, sa)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace ServiceAccount")

			// Check that SA is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(sa.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace ServiceAccount should be owned by DevWorkspace")
			Expect(sa.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs DevWorkspace Deployment", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			Eventually(func() error {
				deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace Deployment")

			// Check that Deployment is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(deploy.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace Deployment should be owned by DevWorkspace")
			Expect(deploy.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Marks DevWorkspace as Running", func() {
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						"test-url/healthz": {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(workspaceID))

			currDW := &dw.DevWorkspace{}
			Eventually(func() (dw.DevWorkspacePhase, error) {
				err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW)
				if err != nil {
					return "", err
				}
				GinkgoWriter.Printf("Waiting for DevWorkspace to enter running phase -- Phase: %s, Message %s\n", currDW.Status.Phase, currDW.Status.Message)
				return currDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusRunning), "Workspace did not enter Running phase before timeout")

			// Verify DevWorkspace is Running as expected
			Expect(currDW.Status.Message).Should(Equal(currDW.Status.MainUrl))
			runningCondition := conditions.GetConditionByType(currDW.Status.Conditions, dw.DevWorkspaceReady)
			Expect(runningCondition).NotTo(BeNil())
			Expect(runningCondition.Status).Should(Equal(corev1.ConditionTrue))
		})

	})

	Context("Automatic provisioning", func() {
		const testURL = "test-url"

		BeforeEach(func() {
			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						fmt.Sprintf("%s/healthz", testURL): {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Mounts image pull secrets to the DevWorkspace Deployment", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Creating secrets for docker configs")
			dockerCfgSecretName := "test-dockercfg"
			dockerCfg := generateSecret(dockerCfgSecretName, corev1.SecretTypeDockercfg)
			dockerCfg.Labels[constants.DevWorkspacePullSecretLabel] = "true"
			dockerCfg.Data[".dockercfg"] = []byte("{}")
			createObject(dockerCfg)
			defer deleteObject(dockerCfg)

			dockerCfgSecretJsonName := "test-dockercfg-json"
			dockerCfgJson := generateSecret(dockerCfgSecretJsonName, corev1.SecretTypeDockerConfigJson)
			dockerCfgJson.Labels[constants.DevWorkspacePullSecretLabel] = "true"
			dockerCfgJson.Data[".dockerconfigjson"] = []byte("{}")
			createObject(dockerCfgJson)
			defer deleteObject(dockerCfgJson)

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			Expect(deploy.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(corev1.LocalObjectReference{Name: dockerCfgSecretName}))
			Expect(deploy.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(corev1.LocalObjectReference{Name: dockerCfgSecretJsonName}))
		})

		It("Manages git credentials for DevWorkspace", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Creating a secret for git credentials")
			gitCredentialsSecretName := "test-git-credentials"
			gitCredentials := generateSecret(gitCredentialsSecretName, corev1.SecretTypeOpaque)
			gitCredentials.Labels[constants.DevWorkspaceGitCredentialLabel] = "true"
			gitCredentials.Data["credentials"] = []byte("https://username:pat@github.com")

			createObject(gitCredentials)
			defer deleteObject(gitCredentials)

			By("Waiting for DevWorkspaceRouting to be created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(dwrName, testNamespace), dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting should be created")

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			modeReadOnly := int32(0640)
			gitconfigVolumeName := common.AutoMountConfigMapVolumeName("devworkspace-gitconfig")
			gitconfigVolume := corev1.Volume{
				Name: gitconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "devworkspace-gitconfig"},
						DefaultMode:          &modeReadOnly,
					},
				},
			}
			gitConfigVolumeMount := corev1.VolumeMount{
				Name:      gitconfigVolumeName,
				ReadOnly:  true,
				MountPath: "/etc/gitconfig",
				SubPath:   "gitconfig",
			}
			gitCredentialsVolumeName := common.AutoMountSecretVolumeName("devworkspace-merged-git-credentials")
			gitCredentialsVolume := corev1.Volume{
				Name: gitCredentialsVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "devworkspace-merged-git-credentials",
						DefaultMode: &modeReadOnly,
					},
				},
			}
			gitCredentialsVolumeMount := corev1.VolumeMount{
				Name:      gitCredentialsVolumeName,
				ReadOnly:  true,
				MountPath: "/.git-credentials/",
			}

			volumes := deploy.Spec.Template.Spec.Volumes
			Expect(volumes).Should(ContainElements(gitconfigVolume, gitCredentialsVolume), "Git credentials should be mounted as volumes in Deployment")
			for _, container := range deploy.Spec.Template.Spec.Containers {
				Expect(container.VolumeMounts).Should(ContainElements(gitConfigVolumeMount, gitCredentialsVolumeMount))
			}
		})

		It("Automounts secrets and configmaps volumes", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Creating a automount secrets and configmaps")
			fileCM := generateConfigMap("file-cm")
			fileCM.Labels[constants.DevWorkspaceMountLabel] = "true"
			fileCM.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/file/cm"
			createObject(fileCM)
			defer deleteObject(fileCM)

			subpathCM := generateConfigMap("subpath-cm")
			subpathCM.Labels[constants.DevWorkspaceMountLabel] = "true"
			subpathCM.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/subpath/cm"
			subpathCM.Annotations[constants.DevWorkspaceMountAsAnnotation] = "subpath"
			subpathCM.Data["testdata1"] = "testValue"
			subpathCM.Data["testdata2"] = "testValue"
			createObject(subpathCM)
			defer deleteObject(subpathCM)

			fileSecret := generateSecret("file-secret", corev1.SecretTypeOpaque)
			fileSecret.Labels[constants.DevWorkspaceMountLabel] = "true"
			fileSecret.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/file/secret"
			createObject(fileSecret)
			defer deleteObject(fileSecret)

			subpathSecret := generateSecret("subpath-secret", corev1.SecretTypeOpaque)
			subpathSecret.Labels[constants.DevWorkspaceMountLabel] = "true"
			subpathSecret.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/subpath/secret"
			subpathSecret.Annotations[constants.DevWorkspaceMountAsAnnotation] = "subpath"
			subpathSecret.Data["testsecret"] = []byte("testValue")
			createObject(subpathSecret)
			defer deleteObject(subpathSecret)

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			expectedAutomountVolumes := []corev1.Volume{
				volumeFromConfigMap(fileCM),
				volumeFromConfigMap(subpathCM),
				volumeFromSecret(fileSecret),
				volumeFromSecret(subpathSecret),
			}
			Expect(deploy.Spec.Template.Spec.Volumes).Should(ContainElements(expectedAutomountVolumes), "Automount volumes should be added to deployment")
			expectedAutomountVolumeMounts := []corev1.VolumeMount{
				volumeMountFromConfigMap(fileCM, "/file/cm", ""),
				volumeMountFromConfigMap(subpathCM, "/subpath/cm", "testdata1"),
				volumeMountFromConfigMap(subpathCM, "/subpath/cm", "testdata2"),
				volumeMountFromSecret(fileSecret, "/file/secret", ""),
				volumeMountFromSecret(subpathSecret, "/subpath/secret", "testsecret"),
			}
			for _, container := range deploy.Spec.Template.Spec.Containers {
				Expect(container.VolumeMounts).Should(ContainElements(expectedAutomountVolumeMounts), "Automount volumeMounts should be added to all containers")
			}
		})

		It("Automounts secrets and configmaps env vars", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Creating a automount secrets and configmaps")
			cm := generateConfigMap("env-cm")
			cm.Labels[constants.DevWorkspaceMountLabel] = "true"
			cm.Annotations[constants.DevWorkspaceMountAsAnnotation] = "env"
			createObject(cm)
			defer deleteObject(cm)

			secret := generateSecret("env-secret", corev1.SecretTypeOpaque)
			secret.Labels[constants.DevWorkspaceMountLabel] = "true"
			secret.Annotations[constants.DevWorkspaceMountAsAnnotation] = "env"
			createObject(secret)
			defer deleteObject(secret)

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))
			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			expectedEnvFromSources := []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
					},
				},
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
					},
				},
			}
			for _, container := range deploy.Spec.Template.Spec.Containers {
				Expect(container.EnvFrom).Should(ContainElements(expectedEnvFromSources), "Automounted env sources should be added to containers")
			}
		})

		It("Sorts automount secrets in consistent order", func() {
			By("Creating automount secrets in non-sorted order")
			secretZ := generateSecret("secret-z", corev1.SecretTypeOpaque)
			secretZ.Labels[constants.DevWorkspaceMountLabel] = "true"
			secretZ.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/z"
			createObject(secretZ)
			defer deleteObject(secretZ)

			secretA := generateSecret("secret-a", corev1.SecretTypeOpaque)
			secretA.Labels[constants.DevWorkspaceMountLabel] = "true"
			secretA.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/a"
			createObject(secretA)
			defer deleteObject(secretA)

			secretM := generateSecret("secret-m", corev1.SecretTypeOpaque)
			secretM.Labels[constants.DevWorkspaceMountLabel] = "true"
			secretM.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/m"
			createObject(secretM)
			defer deleteObject(secretM)

			// Create secrets with numeric suffixes to test numeric sorting
			secret15 := generateSecret("automount-secret-15", corev1.SecretTypeOpaque)
			secret15.Labels[constants.DevWorkspaceMountLabel] = "true"
			secret15.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/15"
			createObject(secret15)
			defer deleteObject(secret15)

			secret02 := generateSecret("automount-secret-02", corev1.SecretTypeOpaque)
			secret02.Labels[constants.DevWorkspaceMountLabel] = "true"
			secret02.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/02"
			createObject(secret02)
			defer deleteObject(secret02)

			secret08 := generateSecret("automount-secret-08", corev1.SecretTypeOpaque)
			secret08.Labels[constants.DevWorkspaceMountLabel] = "true"
			secret08.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/08"
			createObject(secret08)
			defer deleteObject(secret08)

			By("Creating DevWorkspace")
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			By("Verifying secrets are sorted in deployment volumes")

			expectedSecretNames := []string{"secret-a", "secret-m", "secret-z", "automount-secret-02", "automount-secret-08", "automount-secret-15"}
			var automountVolumes []corev1.Volume
			for _, vol := range deploy.Spec.Template.Spec.Volumes {
				if vol.Secret != nil {
					for _, name := range expectedSecretNames {
						if vol.Name == name && vol.Secret.SecretName == name {
							automountVolumes = append(automountVolumes, vol)
							break
						}
					}
				}
			}

			// Verify we found all expected volumes
			Expect(automountVolumes).Should(HaveLen(6), "Should have 6 automount secret volumes")

			// Verify volumes are in sorted order (alphabetically by volume name, which matches secret name)
			expectedOrder := []string{
				"automount-secret-02",
				"automount-secret-08",
				"automount-secret-15",
				"secret-a",
				"secret-m",
				"secret-z",
			}

			actualOrder := make([]string, len(automountVolumes))
			for i, vol := range automountVolumes {
				actualOrder[i] = vol.Name
			}

			Expect(actualOrder).Should(Equal(expectedOrder), "Automount secret volumes should be sorted alphabetically by volume name")
		})

		It("Sorts automount configmaps in consistent order", func() {
			By("Creating automount configmaps in non-sorted order")
			configmapZ := generateConfigMap("configmap-z")
			configmapZ.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmapZ.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/z"
			createObject(configmapZ)
			defer deleteObject(configmapZ)

			configmapA := generateConfigMap("configmap-a")
			configmapA.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmapA.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/a"
			createObject(configmapA)
			defer deleteObject(configmapA)

			configmapM := generateConfigMap("configmap-m")
			configmapM.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmapM.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/m"
			createObject(configmapM)
			defer deleteObject(configmapM)

			// Create configmaps with numeric suffixes to test numeric sorting
			configmap15 := generateConfigMap("automount-cm-15")
			configmap15.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmap15.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/15"
			createObject(configmap15)
			defer deleteObject(configmap15)

			configmap02 := generateConfigMap("automount-cm-02")
			configmap02.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmap02.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/02"
			createObject(configmap02)
			defer deleteObject(configmap02)

			configmap08 := generateConfigMap("automount-cm-08")
			configmap08.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmap08.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/08"
			createObject(configmap08)
			defer deleteObject(configmap08)

			By("Creating DevWorkspace")
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			By("Verifying configmaps are sorted in deployment volumes")

			expectedConfigMapNames := []string{"configmap-a", "configmap-m", "configmap-z", "automount-cm-02", "automount-cm-08", "automount-cm-15"}
			var automountVolumes []corev1.Volume
			for _, vol := range deploy.Spec.Template.Spec.Volumes {
				if vol.ConfigMap != nil {
					for _, name := range expectedConfigMapNames {
						if vol.Name == name && vol.ConfigMap.Name == name {
							automountVolumes = append(automountVolumes, vol)
							break
						}
					}
				}
			}

			// Verify we found all expected volumes
			Expect(automountVolumes).Should(HaveLen(6), "Should have 6 automount configmap volumes")

			// Verify volumes are in sorted order (alphabetically by volume name, which matches configmap name)
			expectedOrder := []string{
				"automount-cm-02",
				"automount-cm-08",
				"automount-cm-15",
				"configmap-a",
				"configmap-m",
				"configmap-z",
			}

			actualOrder := make([]string, len(automountVolumes))
			for i, vol := range automountVolumes {
				actualOrder[i] = vol.Name
			}

			Expect(actualOrder).Should(Equal(expectedOrder), "Automount configmap volumes should be sorted alphabetically by volume name")
		})

		It("Sorts mixed automount secrets and configmaps together", func() {
			By("Creating automount secrets and configmaps in non-sorted order")
			secretB := generateSecret("secret-b", corev1.SecretTypeOpaque)
			secretB.Labels[constants.DevWorkspaceMountLabel] = "true"
			secretB.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/b"
			createObject(secretB)
			defer deleteObject(secretB)

			secretD := generateSecret("secret-d", corev1.SecretTypeOpaque)
			secretD.Labels[constants.DevWorkspaceMountLabel] = "true"
			secretD.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/secret/d"
			createObject(secretD)
			defer deleteObject(secretD)

			configmapA := generateConfigMap("configmap-a")
			configmapA.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmapA.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/a"
			createObject(configmapA)
			defer deleteObject(configmapA)

			configmapC := generateConfigMap("configmap-c")
			configmapC.Labels[constants.DevWorkspaceMountLabel] = "true"
			configmapC.Annotations[constants.DevWorkspaceMountPathAnnotation] = "/configmap/c"
			createObject(configmapC)
			defer deleteObject(configmapC)

			By("Creating DevWorkspace")
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			By("Verifying secrets and configmaps are sorted together")
			expectedNames := []string{"configmap-a", "configmap-c", "secret-b", "secret-d"}
			var automountVolumes []corev1.Volume
			for _, vol := range deploy.Spec.Template.Spec.Volumes {
				if vol.Secret != nil {
					for _, name := range expectedNames {
						if vol.Name == name && vol.Secret.SecretName == name {
							automountVolumes = append(automountVolumes, vol)
							break
						}
					}
				}
				if vol.ConfigMap != nil {
					for _, name := range expectedNames {
						if vol.Name == name && vol.ConfigMap.Name == name {
							automountVolumes = append(automountVolumes, vol)
							break
						}
					}
				}
			}

			Expect(automountVolumes).Should(HaveLen(4), "Should have 4 automount volumes (2 secrets + 2 configmaps)")

			// All volumes should be sorted together alphabetically
			expectedOrder := []string{
				"configmap-a",
				"configmap-c",
				"secret-b",
				"secret-d",
			}

			actualOrder := make([]string, len(automountVolumes))
			for i, vol := range automountVolumes {
				actualOrder[i] = vol.Name
			}

			Expect(actualOrder).Should(Equal(expectedOrder), "Automount volumes (secrets and configmaps) should be sorted together alphabetically")
		})

		It("Detects changes to automount resources and reconciles", func() {
			// NOTE: timeout for this test is reduced, as eventually DWO will reconcile the workspace by coincidence and notice
			// the automount secret.
			createStartedDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			mergedSecretNN := namespacedName(constants.GitCredentialsMergedSecretName, testNamespace)
			mergedSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, mergedSecretNN, mergedSecret)).Error()

			By("Creating git-credential secret")
			secret := generateSecret("git-credential-secret", corev1.SecretTypeOpaque)
			secret.Labels[constants.DevWorkspaceGitCredentialLabel] = "true"
			secret.Data["credentials"] = []byte("https://test:token@github.com")
			createObject(secret)
			defer deleteObject(secret)

			By("Checking that merged credentials secret is created")
			Eventually(func() error {
				return k8sClient.Get(ctx, mergedSecretNN, mergedSecret)
			}, 1*time.Second, interval).Should(Succeed(), "Merged credentials secret is created")

			By("Checking that workspace deployment mounts merged credentials secret")
			Eventually(func() error {
				deploy := &appsv1.Deployment{}
				deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
				if err := k8sClient.Get(ctx, deployNN, deploy); err != nil {
					return err
				}
				for _, volume := range deploy.Spec.Template.Spec.Volumes {
					if volume.Secret != nil && volume.Secret.SecretName == constants.GitCredentialsMergedSecretName {
						return nil
					}
				}
				return fmt.Errorf("Secret not found in volumes")
			}, 1*time.Second, interval).Should(Succeed(), "Merged credentials secret is added to deployment")
		})
	})

	Context("DevWorkspace deployment", func() {
		const testURL = "test-url"

		BeforeEach(func() {
			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						fmt.Sprintf("%s/healthz", testURL): {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Sets the runtimeClassName from the DWOC", func() {
			By("Creating DWOC with a runtimeClassName")
			runtimeClassName := "test-runtime-class"
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					RuntimeClassName: &runtimeClassName,
				},
			})
			defer config.SetGlobalConfigForTesting(nil)

			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")
			Expect(*deploy.Spec.Template.Spec.RuntimeClassName).Should(Equal("test-runtime-class"))
		})

		It("Sets the runtimeClassName from the attribute instead of the DWOC", func() {
			By("Creating DWOC with a runtimeClassName")
			runtimeClassName := "test-runtime-class"
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					RuntimeClassName: &runtimeClassName,
				},
			})
			defer config.SetGlobalConfigForTesting(nil)

			devworkspace := &dw.DevWorkspace{}
			createDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")

			By("Adding runtime-class attribute to the DevWorkspace")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				devworkspace.Spec.Template.Attributes.PutString(constants.RuntimeClassNameAttribute, "test-runtime-class-attribute")
				return k8sClient.Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "DevWorkspace should get `runtime-class: test-runtime-class-attribute` attribute")

			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := namespacedName(common.DeploymentName(workspaceID), testNamespace)
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")
			Expect(*deploy.Spec.Template.Spec.RuntimeClassName).Should(Equal("test-runtime-class-attribute"))
		})
	})

	Context("Stopping DevWorkspaces", func() {
		const testURL = "test-url"

		BeforeEach(func() {
			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						fmt.Sprintf("%s/healthz", testURL): {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
			createStartedDevWorkspace(devWorkspaceName, "test-devworkspace.yaml")
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Stops workspaces and scales deployment to zero", func() {
			devworkspace := &dw.DevWorkspace{}

			By("Setting DevWorkspace's .spec.started to false")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				devworkspace.Spec.Started = false
				return k8sClient.Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "Update DevWorkspace to have .spec.started = false")

			By("Adds devworkspace-started annotation to false on DevWorkspaceRouting")
			Eventually(func() (string, error) {
				dwr := &controllerv1alpha1.DevWorkspaceRouting{}
				if err := k8sClient.Get(ctx, namespacedName(common.DevWorkspaceRoutingName(devworkspace.Status.DevWorkspaceId), testNamespace), dwr); err != nil {
					return "", err
				}
				annotation, ok := dwr.Annotations[constants.DevWorkspaceStartedStatusAnnotation]
				if !ok {
					return "", fmt.Errorf("%s annotation not present", constants.DevWorkspaceStartedStatusAnnotation)
				}
				return annotation, nil
			}, timeout, interval).Should(Equal("false"), "DevWorkspace Routing should get `devworkspace-started: false` annotation")

			By("Checking that workspace deployment is scaled to zero")
			Eventually(func() (replicas int32, err error) {
				deploy := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, namespacedName(common.DeploymentName(devworkspace.Status.DevWorkspaceId), testNamespace), deploy); err != nil {
					return -1, err
				}
				return *deploy.Spec.Replicas, nil
			}, timeout, interval).Should(Equal(int32(0)), "Workspace deployment was not scaled to zero")

			By("Setting DevWorkspace's deployment replicas to zero")
			scaleDeploymentToZero(common.DeploymentName(devworkspace.Status.DevWorkspaceId))

			currDW := &dw.DevWorkspace{}
			Eventually(func() (dw.DevWorkspacePhase, error) {
				if err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW); err != nil {
					return "", err
				}
				GinkgoWriter.Printf("Waiting for DevWorkspace to enter Stopped phase -- Phase: %s, Message %s\n", currDW.Status.Phase, currDW.Status.Message)
				return currDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusStopped), "Workspace did not enter Stopped phase before timeout")

			Expect(currDW.Status.Message).Should(Equal("Stopped"))
			startedCondition := conditions.GetConditionByType(currDW.Status.Conditions, conditions.Started)
			Expect(startedCondition).Should(Not(BeNil()), "Workspace should have Started condition")
			Expect(startedCondition.Status).Should(Equal(corev1.ConditionFalse), "Workspace Started condition should have status=false")
			Expect(startedCondition.Message).Should(Equal("Workspace is stopped"))
		})

		It("Stops workspaces and deletes resources when cleanup option is enabled", func() {
			boolTrue := true
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					CleanupOnStop: &boolTrue,
				},
			})
			defer config.SetGlobalConfigForTesting(nil)
			devworkspace := &dw.DevWorkspace{}

			By("Setting DevWorkspace's .spec.started to false")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				devworkspace.Spec.Started = false
				return k8sClient.Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "Update DevWorkspace to have .spec.started = false")
			workspaceId := devworkspace.Status.DevWorkspaceId

			By("Checking that workspace owned objects are deleted")
			objects := []client.Object{
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: common.DeploymentName(workspaceId)}},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: common.MetadataConfigMapName(workspaceId)}},
				&controllerv1alpha1.DevWorkspaceRouting{ObjectMeta: metav1.ObjectMeta{Name: common.DevWorkspaceRoutingName(workspaceId)}},
			}
			for _, obj := range objects {
				Eventually(func() error {
					err := k8sClient.Get(ctx, namespacedName(obj.GetName(), testNamespace), obj)
					switch {
					case err == nil:
						return fmt.Errorf("Object exists")
					case k8sErrors.IsNotFound(err):
						return nil
					default:
						return err
					}
				}, timeout, interval).Should(Succeed(), "DevWorkspace-owned %s should be deleted", obj.GetObjectKind().GroupVersionKind().Kind)
			}

			currDW := &dw.DevWorkspace{}
			Eventually(func() (dw.DevWorkspacePhase, error) {
				if err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW); err != nil {
					return "", err
				}
				GinkgoWriter.Printf("Waiting for DevWorkspace to enter Stopped phase -- Phase: %s, Message %s\n", currDW.Status.Phase, currDW.Status.Message)
				return currDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusStopped), "Workspace did not enter Stopped phase before timeout")

			Expect(currDW.Status.Message).Should(Equal("Stopped"))
			startedCondition := conditions.GetConditionByType(currDW.Status.Conditions, conditions.Started)
			Expect(startedCondition).Should(Not(BeNil()), "Workspace should have Started condition")
			Expect(startedCondition.Status).Should(Equal(corev1.ConditionFalse), "Workspace Started condition should have status=false")
			Expect(startedCondition.Message).Should(Equal("Workspace is stopped"))
		})

		It("Stops failing workspaces with debug annotation after timeout", func() {
			devworkspace := &dw.DevWorkspace{}
			failTime := metav1.Time{Time: clock.Now().Add(-20 * time.Second)}

			By("Set debug start annotation on DevWorkspace")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				if devworkspace.Annotations == nil {
					devworkspace.Annotations = map[string]string{}
				}
				devworkspace.Annotations[constants.DevWorkspaceDebugStartAnnotation] = "true"
				return k8sClient.Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "Should be able to set failing status on DevWorkspace")

			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					ProgressTimeout: "1s",
				},
			})
			defer config.SetGlobalConfigForTesting(nil)

			By("Setting failing phase on workspace directly")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				devworkspace.Status.Phase = "Failing"
				devworkspace.Status.Conditions = append(devworkspace.Status.Conditions, dw.DevWorkspaceCondition{
					Type:               dw.DevWorkspaceFailedStart,
					LastTransitionTime: failTime,
					Status:             corev1.ConditionTrue,
					Message:            "testing failed condition",
				})
				return k8sClient.Status().Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "Should be able to set failing status on DevWorkspace")

			currDW := &dw.DevWorkspace{}
			Eventually(func() (started bool, err error) {
				if err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW); err != nil {
					return false, err
				}
				return currDW.Spec.Started, nil
			}, timeout, interval).Should(BeFalse(), "DevWorkspace should have spec.started = false")
		})

		It("Stops failing workspaces", func() {
			devworkspace := &dw.DevWorkspace{}

			By("Setting failing phase on workspace directly")
			Eventually(func() error {
				devworkspace = getExistingDevWorkspace(devWorkspaceName)
				devworkspace.Status.Phase = "Failing"
				return k8sClient.Status().Update(ctx, devworkspace)
			}, timeout, interval).Should(Succeed(), "Should be able to set failing status on DevWorkspace")

			currDW := &dw.DevWorkspace{}
			Eventually(func() (started bool, err error) {
				if err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW); err != nil {
					return false, err
				}
				return currDW.Spec.Started, nil
			}, timeout, interval).Should(BeFalse(), "DevWorkspace should have spec.started = false")
		})
	})

	Context("Deleting DevWorkspaces", func() {
		const testURL = "test-url"
		const altDevWorkspaceName = "test-devworkspace-2"

		BeforeEach(func() {
			By("Setting up HTTP client")
			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						fmt.Sprintf("%s/healthz", testURL): {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
		})

		AfterEach(func() {
			By("Deleting DevWorkspaces from test")
			deleteDevWorkspace(devWorkspaceName)
			deleteDevWorkspace(altDevWorkspaceName)
			cleanupPVC("claim-devworkspace")
			By("Resetting HTTP client")
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Cleans up workspace PVC storage when other workspaces exist", func() {
			By("Creating multiple DevWorkspaces")
			createStartedDevWorkspace(devWorkspaceName, "common-pvc-test-devworkspace.yaml")
			createStartedDevWorkspace(altDevWorkspaceName, "common-pvc-test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			Expect(devworkspace.Finalizers).Should(ContainElement(constants.StorageCleanupFinalizer), "DevWorkspace should get storage cleanup finalizer")

			By("Deleting existing workspace")
			Expect(k8sClient.Delete(ctx, devworkspace)).Should(Succeed())

			By("Check that cleanup job is created")
			cleanupJob := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(common.PVCCleanupJobName(devworkspace.Status.DevWorkspaceId), testNamespace), cleanupJob)
			}, timeout, interval).Should(Succeed(), "Cleanup job should be created when workspace is deleted")
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(cleanupJob.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Routing should be owned by DevWorkspace")
			Expect(cleanupJob.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(devworkspace.Status.DevWorkspaceId), "Object should be labelled with DevWorkspace ID")

			By("Marking Job as successfully completed")
			cleanupJob.Status.Succeeded = 1
			cleanupJob.Status.Conditions = []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			}
			Expect(k8sClient.Status().Update(ctx, cleanupJob)).Should(Succeed(), "Failed to update cleanup job")

			By("Checking that workspace is deleted")
			currDW := &dw.DevWorkspace{}
			Eventually(func() (exists bool) {
				err := k8sClient.Get(ctx, namespacedName(devWorkspaceName, testNamespace), currDW)
				return err != nil && k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "Finalizer should be cleared and workspace should be deleted")
		})

		It("Marks workspace as Errored when cleanup Job fails", func() {
			By("Creating multiple DevWorkspaces")
			createStartedDevWorkspace(devWorkspaceName, "common-pvc-test-devworkspace.yaml")
			createStartedDevWorkspace(altDevWorkspaceName, "common-pvc-test-devworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			Expect(devworkspace.Finalizers).Should(ContainElement(constants.StorageCleanupFinalizer), "DevWorkspace should get storage cleanup finalizer")

			By("Deleting existing workspace")
			Expect(k8sClient.Delete(ctx, devworkspace)).Should(Succeed())

			By("Check that cleanup job is created")
			cleanupJob := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(common.PVCCleanupJobName(devworkspace.Status.DevWorkspaceId), testNamespace), cleanupJob)
			}, timeout, interval).Should(Succeed(), "Cleanup job should be created when workspace is deleted")
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(cleanupJob.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Routing should be owned by DevWorkspace")
			Expect(cleanupJob.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(devworkspace.Status.DevWorkspaceId), "Object should be labelled with DevWorkspace ID")

			By("Marking Job as failed")
			cleanupJob.Status.Conditions = []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			}
			Expect(k8sClient.Status().Update(ctx, cleanupJob)).Should(Succeed(), "Failed to update cleanup job")

			By("Checking that workspace is not deleted and ends up in error state")
			currDW := &dw.DevWorkspace{}
			Eventually(func() (dw.DevWorkspacePhase, error) {
				if err := k8sClient.Get(ctx, namespacedName(devWorkspaceName, testNamespace), currDW); err != nil {
					return "", err
				}
				return currDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusError), "DevWorkspace should enter error phase")
			Expect(currDW.Finalizers).Should(ContainElement(constants.StorageCleanupFinalizer))
		})

		It("Deletes shared PVC and clears finalizers when all workspaces are deleted", func() {
			By("Creating DevWorkspaces")
			createStartedDevWorkspace(devWorkspaceName, "common-pvc-test-devworkspace.yaml")
			devworkspace1 := getExistingDevWorkspace(devWorkspaceName)
			Expect(devworkspace1.Finalizers).Should(ContainElement(constants.StorageCleanupFinalizer), "DevWorkspace should get storage cleanup finalizer")

			createStartedDevWorkspace(altDevWorkspaceName, "common-pvc-test-devworkspace.yaml")
			devworkspace2 := getExistingDevWorkspace(altDevWorkspaceName)
			Expect(devworkspace2.Finalizers).Should(ContainElement(constants.StorageCleanupFinalizer), "DevWorkspace should get storage cleanup finalizer")

			By("Deleting existing workspaces")
			Expect(k8sClient.Delete(ctx, devworkspace1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, devworkspace2)).Should(Succeed())

			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() (deleted bool, err error) {
				if err := k8sClient.Get(ctx, namespacedName("claim-devworkspace", testNamespace), pvc); err != nil {
					return false, err
				}
				return pvc.DeletionTimestamp != nil, nil
			}, timeout, interval).Should(BeTrue(), "Shared PVC should be deleted")

			By(fmt.Sprintf("Checking that devworkspace %s is deleted", devWorkspaceName))
			currDW := &dw.DevWorkspace{}
			Eventually(func() (exists bool) {
				err := k8sClient.Get(ctx, namespacedName(devWorkspaceName, testNamespace), currDW)
				return err != nil && k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "Finalizer should be cleared and workspace should be deleted")

			By(fmt.Sprintf("Checking that devworkspace %s is deleted", altDevWorkspaceName))
			Eventually(func() (exists bool) {
				err := k8sClient.Get(ctx, namespacedName(altDevWorkspaceName, testNamespace), currDW)
				return err != nil && k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "Finalizer should be cleared and workspace should be deleted")
		})
	})

	Context("Workspace Restore", func() {
		const testURL = "test-url"

		BeforeEach(func() {
			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						fmt.Sprintf("%s/healthz", testURL): {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Restores workspace from backup with common PVC", func() {
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
						Enable: ptr.To[bool](true),
						Registry: &controllerv1alpha1.RegistryConfig{
							Path: "localhost:5000",
						},
					},
				},
			})
			defer config.SetGlobalConfigForTesting(nil)
			By("Reading DevWorkspace with restore configuration from testdata file")
			createDevWorkspace(devWorkspaceName, "restore-workspace-common.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Waiting for DevWorkspaceRouting to be created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(dwrName, testNamespace), dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting should be created")

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(devworkspace.Status.DevWorkspaceId))

			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, namespacedName(devworkspace.Status.DevWorkspaceId, devworkspace.Namespace), deployment)
			Expect(err).ToNot(HaveOccurred(), "Failed to get DevWorkspace deployment")

			initContainers := deployment.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(BeNumerically(">", 0), "No initContainers found in deployment")

			var restoreInitContainer corev1.Container
			var cloneInitContainer corev1.Container
			for _, container := range initContainers {
				if container.Name == restore.WorkspaceRestoreContainerName {
					restoreInitContainer = container
				}
				if container.Name == projects.ProjectClonerContainerName {
					cloneInitContainer = container
				}
			}
			Expect(cloneInitContainer.Name).To(BeEmpty(), "Project clone init container should be omitted when restoring from backup")
			Expect(restoreInitContainer).ToNot(BeNil(), "Workspace restore init container should not be nil")
			Expect(restoreInitContainer.Name).To(Equal(restore.WorkspaceRestoreContainerName), "Workspace restore init container should be present in deployment")

			Expect(restoreInitContainer.Command).To(Equal([]string{"/workspace-recovery.sh"}), "Restore init container should have correct command")
			Expect(restoreInitContainer.Args).To(Equal([]string{"--restore"}), "Restore init container should have correct args")
			Expect(restoreInitContainer.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:        "claim-devworkspace", // PVC name for common storage
				MountPath:   constants.DefaultProjectsSourcesRoot,
				ReadOnly:    false,
				SubPath:     workspaceID + "/projects", // Dynamic workspace ID + projects
				SubPathExpr: "",
			}), "Restore init container should have workspace storage volume mounted at correct path")

		})
		It("Restores workspace from backup with per-workspace PVC", func() {
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
						Enable: ptr.To[bool](true),
						Registry: &controllerv1alpha1.RegistryConfig{
							Path: "localhost:5000",
						},
					},
				},
			})
			defer config.SetGlobalConfigForTesting(nil)
			By("Reading DevWorkspace with restore configuration from testdata file")
			createDevWorkspace(devWorkspaceName, "restore-workspace-perworkspace.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Waiting for DevWorkspaceRouting to be created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(dwrName, testNamespace), dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting should be created")

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(devworkspace.Status.DevWorkspaceId))

			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, namespacedName(devworkspace.Status.DevWorkspaceId, devworkspace.Namespace), deployment)
			Expect(err).ToNot(HaveOccurred(), "Failed to get DevWorkspace deployment")

			initContainers := deployment.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(BeNumerically(">", 0), "No initContainers found in deployment")

			var restoreInitContainer corev1.Container
			var cloneInitContainer corev1.Container
			for _, container := range initContainers {
				if container.Name == restore.WorkspaceRestoreContainerName {
					restoreInitContainer = container
				}
				if container.Name == projects.ProjectClonerContainerName {
					cloneInitContainer = container
				}
			}
			Expect(cloneInitContainer.Name).To(BeEmpty(), "Project clone init container should be omitted when restoring from backup")
			Expect(restoreInitContainer).ToNot(BeNil(), "Workspace restore init container should not be nil")
			Expect(restoreInitContainer.Name).To(Equal(restore.WorkspaceRestoreContainerName), "Workspace restore init container should be present in deployment")

			Expect(restoreInitContainer.Command).To(Equal([]string{"/workspace-recovery.sh"}), "Restore init container should have correct command")
			Expect(restoreInitContainer.Args).To(Equal([]string{"--restore"}), "Restore init container should have correct args")
			Expect(restoreInitContainer.VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:        common.PerWorkspacePVCName(workspaceID),
				MountPath:   constants.DefaultProjectsSourcesRoot,
				ReadOnly:    false,
				SubPath:     "projects",
				SubPathExpr: "",
			}), "Restore init container should have workspace storage volume mounted at correct path")

		})
		It("Doesn't restore workspace from backup if restore is disabled", func() {
			config.SetGlobalConfigForTesting(&controllerv1alpha1.OperatorConfiguration{
				Workspace: &controllerv1alpha1.WorkspaceConfig{
					BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
						Enable: ptr.To[bool](true),
						Registry: &controllerv1alpha1.RegistryConfig{
							Path: "localhost:5000",
						},
					},
				},
			})
			defer config.SetGlobalConfigForTesting(nil)
			By("Reading DevWorkspace with restore configuration from testdata file")
			createDevWorkspace(devWorkspaceName, "restore-workspace-disabled.yaml")
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Waiting for DevWorkspaceRouting to be created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(dwrName, testNamespace), dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting should be created")

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(devworkspace.Status.DevWorkspaceId))

			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, namespacedName(devworkspace.Status.DevWorkspaceId, devworkspace.Namespace), deployment)
			Expect(err).ToNot(HaveOccurred(), "Failed to get DevWorkspace deployment")

			initContainers := deployment.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(BeNumerically(">", 0), "No initContainers found in deployment")

			var restoreInitContainer corev1.Container
			var cloneInitContainer corev1.Container
			for _, container := range initContainers {
				if container.Name == restore.WorkspaceRestoreContainerName {
					restoreInitContainer = container
				}
				if container.Name == projects.ProjectClonerContainerName {
					cloneInitContainer = container
				}
			}
			Expect(restoreInitContainer.Name).To(BeEmpty(), "Workspace restore init container should be omitted when restore is disabled")
			Expect(cloneInitContainer).ToNot(BeNil(), "Project clone init container should not be nil")

		})

	})

	Context("Edge cases", func() {

		It("Allows Kubernetes and Container components to share same target port on endpoint", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace-duplicate-ports.yaml")
			defer deleteDevWorkspace(devWorkspaceName)
			devworkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devworkspace.Status.DevWorkspaceId

			workspacecontroller.SetupHttpClientsForTesting(&http.Client{
				Transport: &testutil.TestRoundTripper{
					Data: map[string]testutil.TestResponse{
						"test-url/healthz": {
							StatusCode: http.StatusOK,
						},
					},
				},
			})
			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(workspaceID))

			currDW := &dw.DevWorkspace{}
			Eventually(func() (dw.DevWorkspacePhase, error) {
				err := k8sClient.Get(ctx, namespacedName(devworkspace.Name, devworkspace.Namespace), currDW)
				if err != nil {
					return "", err
				}
				GinkgoWriter.Printf("Waiting for DevWorkspace to enter running phase -- Phase: %s, Message %s\n", currDW.Status.Phase, currDW.Status.Message)
				return currDW.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusRunning), "Workspace did not enter Running phase before timeout")

			// Verify DevWorkspace is Running as expected
			Expect(currDW.Status.Message).Should(Equal(currDW.Status.MainUrl))
			runningCondition := conditions.GetConditionByType(currDW.Status.Conditions, dw.DevWorkspaceReady)
			Expect(runningCondition).NotTo(BeNil())
			Expect(runningCondition.Status).Should(Equal(corev1.ConditionTrue))

			// Clean up
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Ensures preStart initContainers are run after project-clone", func() {
			createDevWorkspace(devWorkspaceName, "test-devworkspace-prestart.yaml")
			defer deleteDevWorkspace(devWorkspaceName)
			devWorkspace := getExistingDevWorkspace(devWorkspaceName)
			workspaceID := devWorkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			By("Setting the deployment to have 1 ready replica")
			markDeploymentReady(common.DeploymentName(workspaceID))

			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, namespacedName(workspaceID, testNamespace), deployment)
			Expect(err).ToNot(HaveOccurred(), "Failed to get DevWorkspace deployment")

			By("Checking initContainer order in the created Deployment")
			initContainers := deployment.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(BeNumerically(">", 0), "No initContainers found")

			expectedOrder := []string{
				"project-clone",
				"go-mod-builder",
				"go-builder",
			}

			for i, name := range expectedOrder {
				if i >= len(initContainers) {
					Fail(fmt.Sprintf("Expected init container %s at position %d, but only %d containers found", name, i, len(initContainers)))
				}
				Expect(initContainers[i].Name).To(Equal(name), fmt.Sprintf("Init container at position %d is not %s", i, name))
			}
		})
	})

})
