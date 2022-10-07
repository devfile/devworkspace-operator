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

package controllers_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	workspacecontroller "github.com/devfile/devworkspace-operator/controllers/workspace"
	"github.com/devfile/devworkspace-operator/controllers/workspace/internal/testutil"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
			err := loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")
			Expect(err).NotTo(HaveOccurred())

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := types.NamespacedName{
				Namespace: testNamespace,
				Name:      devWorkspaceName,
			}
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
			err := loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")
			Expect(err).NotTo(HaveOccurred())

			if devworkspace.Annotations == nil {
				devworkspace.Annotations = map[string]string{}
			}
			devworkspace.Annotations[constants.WorkspaceIdOverrideAnnotation] = "test-workspace-id"

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := types.NamespacedName{
				Namespace: testNamespace,
				Name:      devWorkspaceName,
			}
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
			err := loadObjectFromFile(devWorkspaceName, devworkspace, "test-devworkspace.yaml")
			Expect(err).NotTo(HaveOccurred())

			if devworkspace.Annotations == nil {
				devworkspace.Annotations = map[string]string{}
			}
			devworkspace.Annotations[constants.WorkspaceIdOverrideAnnotation] = "test-workspace-id"

			devworkspace2 := devworkspace.DeepCopy()
			devworkspace2.Name = fmt.Sprintf("%s-dupe", devworkspace2.Name)

			By("Creating a new DevWorkspace")
			Expect(k8sClient.Create(ctx, devworkspace)).Should(Succeed())
			dwNamespacedName := types.NamespacedName{
				Namespace: testNamespace,
				Name:      devWorkspaceName,
			}
			defer deleteDevWorkspace(devWorkspaceName)

			createdDW := &dw.DevWorkspace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dwNamespacedName, createdDW)
				return err == nil
			}, timeout, interval).Should(BeTrue(), "DevWorkspace should exist in cluster")

			By("Creating a DevWorkspace that duplicates the workspace ID of the first")
			Expect(k8sClient.Create(ctx, devworkspace2)).Should(Succeed())
			defer deleteDevWorkspace(devworkspace2.Name)

			By("Checking that duplicate DevWorkspace enters failed Phase")
			createdDW2 := &dw.DevWorkspace{}
			Eventually(func() (phase dw.DevWorkspacePhase, err error) {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      devworkspace2.Name,
					Namespace: testNamespace,
				}, createdDW2); err != nil {
					return "", err
				}
				return createdDW2.Status.Phase, nil
			}, timeout, interval).Should(Equal(dw.DevWorkspaceStatusFailed), "DevWorkspace with duplicate ID should fail to start")
		})
	})

	Context("Workspace Objects creation", func() {

		BeforeEach(func() {
			createDevWorkspace("test-devworkspace.yaml")
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Creates roles and rolebindings", func() {
			By("Checking that common role is created")
			dwRole := &rbacv1.Role{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: common.WorkspaceRoleName(), Namespace: testNamespace}, dwRole); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common Role should be created for DevWorkspace")

			By("Checking that common rolebinding is created")
			dwRoleBinding := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: common.WorkspaceRolebindingName(), Namespace: testNamespace}, dwRoleBinding); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue(), "Common RoleBinding should be created for DevWorkspace")
			Expect(dwRoleBinding.RoleRef.Name).Should(Equal(dwRole.Name), "Rolebinding should refer to DevWorkspace role")
			expectedSubject := rbacv1.Subject{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     fmt.Sprintf("system:serviceaccounts:%s", testNamespace),
			}
			Expect(dwRoleBinding.Subjects).Should(ContainElement(expectedSubject), "Rolebinding should bind to serviceaccounts in current namespace")
		})

		It("Creates DevWorkspaceRouting", func() {
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Checking that DevWorkspaceRouting is created")
			dwr := &controllerv1alpha1.DevWorkspaceRouting{}
			dwrName := common.DevWorkspaceRoutingName(workspaceID)
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: dwrName, Namespace: testNamespace}, dwr)
			}, timeout, interval).Should(Succeed(), "DevWorkspaceRouting is present on cluster")

			Expect(string(dwr.Spec.RoutingClass)).Should(Equal(devworkspace.Spec.RoutingClass), "RoutingClass should be propagated to DevWorkspaceRouting")
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(dwr.OwnerReferences).Should(ContainElement(expectedOwnerReference), "Routing should be owned by DevWorkspace")
			Expect(dwr.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs Routing mainURL to DevWorkspace", func() {
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			Eventually(func() (string, error) {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: devWorkspaceName, Namespace: testNamespace}, devworkspace); err != nil {
					return "", err
				}
				return devworkspace.Status.MainUrl, nil
			}, timeout, interval).Should(Equal("test-url"))

		})

		It("Creates workspace metadata configmap", func() {
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			metadataCM := &corev1.ConfigMap{}
			Eventually(func() error {
				cmNN := types.NamespacedName{
					Name:      common.MetadataConfigMapName(workspaceID),
					Namespace: testNamespace,
				}
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
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				saNN := types.NamespacedName{
					Name:      common.ServiceAccountName(workspaceID),
					Namespace: testNamespace,
				}
				return k8sClient.Get(ctx, saNN, sa)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace ServiceAccount")

			// Check that SA is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(sa.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace ServiceAccount should be owned by DevWorkspace")
			Expect(sa.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Syncs DevWorkspace Deployment", func() {
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Manually making Routing ready to continue")
			markRoutingReady("test-url", common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			Eventually(func() error {
				deployNN := types.NamespacedName{
					Name:      common.DeploymentName(workspaceID),
					Namespace: testNamespace,
				}
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Should create DevWorkspace Deployment")

			// Check that Deployment is set up properly
			expectedOwnerReference := devworkspaceOwnerRef(devworkspace)
			Expect(deploy.OwnerReferences).Should(ContainElement(expectedOwnerReference), "DevWorkspace Deployment should be owned by DevWorkspace")
			Expect(deploy.Labels[constants.DevWorkspaceIDLabel]).Should(Equal(workspaceID), "Object should be labelled with DevWorkspace ID")
		})

		It("Marks DevWorkspace as Running", func() {
			devworkspace := getExistingDevWorkspace()
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
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      devworkspace.Name,
					Namespace: devworkspace.Namespace,
				}, currDW)
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
			createDevWorkspace("test-devworkspace.yaml")
		})

		AfterEach(func() {
			deleteDevWorkspace(devWorkspaceName)
			workspacecontroller.SetupHttpClientsForTesting(getBasicTestHttpClient())
		})

		It("Mounts image pull secrets to the DevWorkspace Deployment", func() {
			devworkspace := getExistingDevWorkspace()
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
			deployNN := types.NamespacedName{
				Name:      common.DeploymentName(workspaceID),
				Namespace: testNamespace,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, deployNN, deploy)
			}, timeout, interval).Should(Succeed(), "Getting workspace deployment from cluster")

			Expect(deploy.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(corev1.LocalObjectReference{Name: dockerCfgSecretName}))
			Expect(deploy.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(corev1.LocalObjectReference{Name: dockerCfgSecretJsonName}))
		})

		It("Manages git credentials for DevWorkspace", func() {
			devworkspace := getExistingDevWorkspace()
			workspaceID := devworkspace.Status.DevWorkspaceId

			By("Creating a secret for git credentials")
			gitCredentialsSecretName := "test-git-credentials"
			gitCredentials := generateSecret(gitCredentialsSecretName, corev1.SecretTypeOpaque)
			gitCredentials.Labels[constants.DevWorkspaceGitCredentialLabel] = "true"
			gitCredentials.Data["credentials"] = []byte("https://username:pat@github.com")

			createObject(gitCredentials)
			defer deleteObject(gitCredentials)

			By("Manually making Routing ready to continue")
			markRoutingReady(testURL, common.DevWorkspaceRoutingName(workspaceID))

			deploy := &appsv1.Deployment{}
			deployNN := types.NamespacedName{
				Name:      common.DeploymentName(workspaceID),
				Namespace: testNamespace,
			}
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
			devworkspace := getExistingDevWorkspace()
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
			deployNN := types.NamespacedName{
				Name:      common.DeploymentName(workspaceID),
				Namespace: testNamespace,
			}
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
			devworkspace := getExistingDevWorkspace()
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
			deployNN := types.NamespacedName{
				Name:      common.DeploymentName(workspaceID),
				Namespace: testNamespace,
			}
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
	})

})
