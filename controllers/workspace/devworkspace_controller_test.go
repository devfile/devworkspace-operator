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
	"os"
	"path/filepath"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

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

	})
})
