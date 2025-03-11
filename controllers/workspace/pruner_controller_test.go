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

package controllers_test

import (
	"fmt"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	workspacecontroller "github.com/devfile/devworkspace-operator/controllers/workspace"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Pruner Controller", func() {
	const (
		testPrunerConfigMapName = "test-pruner-configmap"
	)

	Context("Pruner Resources creation", func() {

		AfterEach(func() {
			// Clean up resources created by the test
			deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")
			deleteConfigMap(workspacecontroller.PrunerConfigMap)
			deleteConfigMap(testPrunerConfigMapName)
			deleteCronJob(workspacecontroller.PrunerCronJobName)
			deleteServiceAccount(workspacecontroller.PrunerServiceAccountName)
			deleteClusterRole(workspacecontroller.PrunerClusterRoleName)
			deleteClusterRoleBinding(workspacecontroller.PrunerClusterRoleBindingName)
		})

		It("Creates CronJob and ConfigMap when CleanupCronJob is enabled in DevWorkspaceOperatorConfig", func() {
			// Create a DevWorkspaceOperatorConfig with CleanupCronJob enabled
			enable := true
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "devworkspace-operator-config",
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable: &enable,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dwoc)).Should(Succeed())
			defer deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")

			By("Checking that default ConfigMap is created")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerConfigMap, testNamespace), cm)
			}, timeout, interval).Should(Succeed(), "Default ConfigMap should be created")

			By("Checking that CronJob is created")
			cronJob := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
			}, timeout, interval).Should(Succeed(), "CronJob should be created")

			By("Checking that ServiceAccount is created")
			sa := &corev1.ServiceAccount{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerServiceAccountName, testNamespace), sa)
			}, timeout, interval).Should(Succeed(), "ServiceAccount should be created")

			By("Checking that ClusterRole is created")
			cr := &rbacv1.ClusterRole{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: workspacecontroller.PrunerClusterRoleName}, cr)
			}, timeout, interval).Should(Succeed(), "ClusterRole should be created")

			By("Checking that ClusterRoleBinding is created")
			crb := &rbacv1.ClusterRoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: workspacecontroller.PrunerClusterRoleBindingName}, crb)
			}, timeout, interval).Should(Succeed(), "ClusterRoleBinding should be created")
		})

		It("Creates CronJob with custom image and retainTime and custom configmap when provided in DevWorkspaceOperatorConfig", func() {
			// Create a DevWorkspaceOperatorConfig with CleanupCronJob enabled and custom image, retainTime and configmap
			enabled := true
			customImage := "test-image"
			customRetainTime := int32(12345)
			customConfigMapName := testPrunerConfigMapName

			// create custom configmap
			customConfigMapLabels := constants.ControllerAppLabels()
			customConfigMapLabels["app.kubernetes.io/name"] = "devworkspace-pruner"
			customConfigMapLabels[constants.DevWorkspaceWatchConfigMapLabel] = "true"
			customConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      customConfigMapName,
					Namespace: testNamespace,
					Labels:    customConfigMapLabels,
				},
				Data: map[string]string{
					"devworkspace-pruner": "test-script",
				},
			}
			Expect(k8sClient.Create(ctx, customConfigMap)).Should(Succeed())
			// defer deleteConfigMap(customConfigMapName)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "devworkspace-operator-config",
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable:        &enabled,
							Image:         customImage,
							RetainTime:    &customRetainTime,
							CronJobScript: customConfigMapName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dwoc)).Should(Succeed())
			defer deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")

			By("Checking that default ConfigMap is created")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerConfigMap, testNamespace), cm)
			}, timeout, interval).Should(Succeed(), "Default ConfigMap should be created")

			By("Checking that CronJob is created with custom parameters")
			cronJob := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
			}, timeout, interval).Should(Succeed(), "CronJob should be created")

			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).Should(Equal(customImage), "CronJob should have custom image")
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name).Should(Equal(customConfigMapName), "CronJob should have custom configmap")

			// Check if RETAIN_TIME env var has correct value
			found := false
			for _, env := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "RETAIN_TIME" {
					Expect(env.Value).Should(Equal(fmt.Sprintf("%d", customRetainTime)), "CronJob should have custom retainTime")
					found = true
					break
				}
			}
			Expect(found).Should(BeTrue(), "CronJob should have RETAIN_TIME env var")
		})

		It("Does not create CronJob and ConfigMap when CleanupCronJob is disabled in DevWorkspaceOperatorConfig", func() {
			// Create a DevWorkspaceOperatorConfig with CleanupCronJob disabled
			disabled := false
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "devworkspace-operator-config",
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable: &disabled,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dwoc)).Should(Succeed())
			defer deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")

			By("Checking that ConfigMap is not created")
			cm := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerConfigMap, testNamespace), cm)
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "ConfigMap should not be created")

			By("Checking that CronJob is not created")
			cronJob := &batchv1.CronJob{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "CronJob should not be created")

			By("Checking that ServiceAccount is not created")
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerServiceAccountName, testNamespace), sa)
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "ServiceAccount should not be created")

			By("Checking that ClusterRole is not created")
			cr := &rbacv1.ClusterRole{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: workspacecontroller.PrunerClusterRoleName}, cr)
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "ClusterRole should not be created")

			By("Checking that ClusterRoleBinding is not created")
			crb := &rbacv1.ClusterRoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: workspacecontroller.PrunerClusterRoleBindingName}, crb)
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "ClusterRoleBinding should not be created")
		})

		It("Updates CronJob when CleanupCronJob parameters are updated in DevWorkspaceOperatorConfig", func() {
			// Create a DevWorkspaceOperatorConfig with CleanupCronJob enabled
			enabled := true
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "devworkspace-operator-config",
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable: &enabled,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dwoc)).Should(Succeed())
			defer deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")

			By("Checking that default ConfigMap is created")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerConfigMap, testNamespace), cm)
			}, timeout, interval).Should(Succeed(), "Default ConfigMap should be created")

			By("Checking that CronJob is created")
			cronJob := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
			}, timeout, interval).Should(Succeed(), "CronJob should be created")

			// Update the DevWorkspaceOperatorConfig with new values
			customImage := "test-image"
			customDryRun := true
			customRetainTime := int32(12345)
			customConfigMapName := testPrunerConfigMapName

			// create custom configmap
			customConfigMapLabels := constants.ControllerAppLabels()
			customConfigMapLabels["app.kubernetes.io/name"] = "devworkspace-pruner"
			customConfigMapLabels[constants.DevWorkspaceWatchConfigMapLabel] = "true"
			customConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      customConfigMapName,
					Namespace: testNamespace,
					Labels:    customConfigMapLabels,
				},
				Data: map[string]string{
					"devworkspace-pruner": "test-script",
				},
			}
			Expect(k8sClient.Create(ctx, customConfigMap)).Should(Succeed())
			defer deleteConfigMap(customConfigMapName)

			dwoc.Config.Workspace.CleanupCronJob.Image = customImage
			dwoc.Config.Workspace.CleanupCronJob.RetainTime = &customRetainTime
			dwoc.Config.Workspace.CleanupCronJob.DryRun = &customDryRun
			dwoc.Config.Workspace.CleanupCronJob.CronJobScript = customConfigMapName

			Expect(k8sClient.Update(ctx, dwoc)).Should(Succeed())

			By("Checking that CronJob is updated with new parameters")
			Eventually(func() error {
				err := k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
				if err != nil {
					return err
				}

				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image != customImage {
					return fmt.Errorf("CronJob image not updated, expected %s, got %s", customImage, cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)
				}

				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name != customConfigMapName {
					return fmt.Errorf("CronJob configmap not updated, expected %s, got %s", customConfigMapName, cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name)
				}

				// Check if RETAIN_TIME env var has correct value
				foundRetainTime := false
				for _, env := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "RETAIN_TIME" {
						if env.Value != fmt.Sprintf("%d", customRetainTime) {
							return fmt.Errorf("CronJob retainTime not updated, expected %d, got %s", customRetainTime, env.Value)
						}
						foundRetainTime = true
						break
					}
				}
				if !foundRetainTime {
					return fmt.Errorf("CronJob should have RETAIN_TIME env var")
				}

				// Check if DRY_RUN env var has correct value
				foundDryRun := false
				for _, env := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "DRY_RUN" {
						if env.Value != "true" {
							return fmt.Errorf("CronJob dryRun not updated, expected true, got %s", env.Value)
						}
						foundDryRun = true
						break
					}
				}
				if !foundDryRun {
					return fmt.Errorf("CronJob should have DRY_RUN env var")
				}

				return nil
			}, timeout, interval).Should(Succeed(), "CronJob should be updated with new parameters")
		})

		It("Suspends CronJob when CleanupCronJob is disabled in DevWorkspaceOperatorConfig", func() {
			// Create a DevWorkspaceOperatorConfig with CleanupCronJob enable
			enable := true
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "devworkspace-operator-config",
					Namespace: testNamespace,
				},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable: &enable,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, dwoc)).Should(Succeed())
			defer deleteDevWorkspaceOperatorConfig("devworkspace-operator-config")

			By("Checking that default ConfigMap is created")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerConfigMap, testNamespace), cm)
			}, timeout, interval).Should(Succeed(), "Default ConfigMap should be created")

			By("Checking that CronJob is created")
			cronJob := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
			}, timeout, interval).Should(Succeed(), "CronJob should be created")

			// Update the DevWorkspaceOperatorConfig to disable CleanupCronJob
			disabled := false
			dwoc.Config.Workspace.CleanupCronJob.Enable = &disabled
			Expect(k8sClient.Update(ctx, dwoc)).Should(Succeed())

			By("Checking that CronJob is suspended")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName(workspacecontroller.PrunerCronJobName, testNamespace), cronJob)
				if err != nil {
					return false
				}
				return *cronJob.Spec.Suspend
			}, timeout, interval).Should(BeTrue(), "CronJob should be suspended")
		})
	})
})

func deleteDevWorkspaceOperatorConfig(name string) {
	dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	_ = k8sClient.Delete(ctx, dwoc)
}

func deleteConfigMap(name string) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	_ = k8sClient.Delete(ctx, cm)
}

func deleteCronJob(name string) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	_ = k8sClient.Delete(ctx, cronJob)
}

func deleteServiceAccount(name string) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
	}
	_ = k8sClient.Delete(ctx, sa)
}

func deleteClusterRole(name string) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_ = k8sClient.Delete(ctx, cr)
}

func deleteClusterRoleBinding(name string) {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_ = k8sClient.Delete(ctx, crb)
}
