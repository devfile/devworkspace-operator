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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron/v3"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
)

var _ = Describe("BackupCronJobReconciler", func() {
	var (
		ctx           context.Context
		fakeClient    client.Client
		reconciler    BackupCronJobReconciler
		nameNamespace types.NamespacedName
		log           logr.Logger
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(controllerv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(dwv2.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		log = zap.New(zap.UseDevMode(true)).WithName("BackupCronJobReconcilerTest")

		reconciler = BackupCronJobReconciler{
			Client: fakeClient,
			Log:    log,
			Scheme: scheme,
			cron:   cron.New(),
		}

		nameNamespace = types.NamespacedName{
			Name:      "devworkspace-operator-config",
			Namespace: "devworkspace-controller",
		}
	})

	AfterEach(func() {
		reconciler.stopCron(log) // Ensure cron is stopped after each test
	})

	Context("Reconcile", func() {
		It("Should do nothing if DevWorkspaceOperatorConfig is not found", func() {
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should not start cron if dwOperatorConfig.Config.Workspace.BackupCronJob is nil", func() {
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: nil,
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should not start cron if received event from different namespace", func() {
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: "other-namespace"},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
							Enable:   pointer.Bool(true),
							Schedule: "* * * * *",
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
				Name:      nameNamespace.Name,
				Namespace: nameNamespace.Namespace,
			}})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should start cron if enabled and schedule is defined", func() {
			enabled := true
			schedule := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
							Enable:   &enabled,
							Schedule: schedule,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(HaveLen(1))
		})

		It("Should update cron schedule if DevWorkspaceOperatorConfig is updated", func() {
			enabled := true
			schedule1 := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
							Enable:   &enabled,
							Schedule: schedule1,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(HaveLen(1))
			entryID := reconciler.cron.Entries()[0].ID

			schedule2 := "1 * * * *"
			dwoc.Config.Workspace.BackupCronJob.Schedule = schedule2
			Expect(fakeClient.Update(ctx, dwoc)).To(Succeed())

			result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(HaveLen(1))
			Expect(reconciler.cron.Entries()[0].ID).NotTo(Equal(entryID))
		})

		It("Should stop cron if DevWorkspaceOperatorConfig is deleted", func() {
			enabled := true
			schedule := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: &controllerv1alpha1.BackupCronJobConfig{
							Enable:   &enabled,
							Schedule: schedule,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(HaveLen(1))

			Expect(fakeClient.Delete(ctx, dwoc)).To(Succeed())

			result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).
				To(HaveLen(0))
		})
	})

	Context("executeBackupSync", func() {
		It("creates a Job for a DevWorkspace stopped within last 30 minutes", func() {
			dw := createDevWorkspace("dw-recent", "ns-a", false, metav1.NewTime(time.Now().Add(-10*time.Minute)))
			dw.Status.Phase = dwv2.DevWorkspaceStatusStopped
			dw.Status.DevWorkspaceId = "id-recent"
			Expect(fakeClient.Create(ctx, dw)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "claim-devworkspace", Namespace: dw.Namespace}}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			Expect(reconciler.executeBackupSync(ctx, log)).To(Succeed())

			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, &client.ListOptions{Namespace: dw.Namespace})).To(Succeed())
			Expect(jobList.Items).To(HaveLen(1))
		})

		It("does not create a Job when the DevWorkspace was stopped beyond time range", func() {
			dw := createDevWorkspace("dw-old", "ns-b", false, metav1.NewTime(time.Now().Add(-60*time.Minute)))
			dw.Status.Phase = dwv2.DevWorkspaceStatusStopped
			dw.Status.DevWorkspaceId = "id-old"
			Expect(fakeClient.Create(ctx, dw)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "claim-devworkspace", Namespace: dw.Namespace}}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			Expect(reconciler.executeBackupSync(ctx, log)).To(Succeed())

			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, &client.ListOptions{Namespace: dw.Namespace})).To(Succeed())
			Expect(jobList.Items).To(HaveLen(0))
		})

		It("does not create a Job for a running DevWorkspace", func() {
			dw := createDevWorkspace("dw-running", "ns-c", true, metav1.NewTime(time.Now().Add(-5*time.Minute)))
			Expect(fakeClient.Create(ctx, dw)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "claim-devworkspace", Namespace: dw.Namespace}}
			Expect(fakeClient.Create(ctx, pvc)).To(Succeed())

			Expect(reconciler.executeBackupSync(ctx, log)).To(Succeed())

			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, &client.ListOptions{Namespace: dw.Namespace})).To(Succeed())
			Expect(jobList.Items).To(HaveLen(0))
		})
	})

})

// Helper function to create a DevWorkspace
func createDevWorkspace(name, namespace string, started bool, lastTransitionTime metav1.Time) *dwv2.DevWorkspace {
	dw := &dwv2.DevWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: dwv2.DevWorkspaceSpec{
			Started: started,
		},
		Status: dwv2.DevWorkspaceStatus{
			Conditions: []dwv2.DevWorkspaceCondition{},
		},
	}

	if !lastTransitionTime.IsZero() {
		condition := dwv2.DevWorkspaceCondition{
			Type:               conditions.Started,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
			Reason:             "Test",
			Message:            "Test",
		}
		if !started {
			condition.Status = corev1.ConditionFalse
		}
		dw.Status.Conditions = append(dw.Status.Conditions, condition)
	}

	return dw
}

var _ = Describe("DevWorkspaceOperatorConfig UpdateFunc Tests", func() {
	var configPredicate predicate.Funcs

	BeforeEach(func() {
		configPredicate = predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return shouldReconcileOnUpdate(e, zap.New(zap.UseDevMode(true)))
			},
		}
	})

	DescribeTable("Testing UpdateFunc for backup configuration changes",
		func(oldBackup, newBackup *controllerv1alpha1.BackupCronJobConfig, expected bool) {
			oldCfg := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: oldBackup,
					},
				},
			}
			newCfg := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						BackupCronJob: newBackup,
					},
				},
			}
			updateEvent := event.UpdateEvent{
				ObjectOld: oldCfg,
				ObjectNew: newCfg,
			}
			result := configPredicate.Update(updateEvent)
			Expect(result).To(Equal(expected))
		},

		Entry("Both nil => no change", nil, nil, false),
		Entry("OldBackup==nil, NewBackup!=nil => changed", nil, &controllerv1alpha1.BackupCronJobConfig{}, true),
		Entry("OldBackup!=nil, NewBackup==nil => changed", &controllerv1alpha1.BackupCronJobConfig{}, nil, true),
		Entry("OldBackup.Enable==nil, NewBackup.Enable==nil => no change",
			&controllerv1alpha1.BackupCronJobConfig{Enable: nil},
			&controllerv1alpha1.BackupCronJobConfig{Enable: nil},
			false,
		),
		Entry("OldBackup.Enable==nil, NewBackup.Enable!=nil => changed",
			&controllerv1alpha1.BackupCronJobConfig{Enable: nil},
			&controllerv1alpha1.BackupCronJobConfig{Enable: pointer.Bool(true)},
			true,
		),
		Entry("OldBackup.Enable!=nil, NewBackup.Enable==nil => changed",
			&controllerv1alpha1.BackupCronJobConfig{Enable: pointer.Bool(true)},
			&controllerv1alpha1.BackupCronJobConfig{Enable: nil},
			true,
		),
		Entry("Enable differs => changed",
			&controllerv1alpha1.BackupCronJobConfig{Enable: pointer.Bool(true)},
			&controllerv1alpha1.BackupCronJobConfig{Enable: pointer.Bool(false)},
			true,
		),
		Entry("Schedule differs => changed",
			&controllerv1alpha1.BackupCronJobConfig{Schedule: "0 * * * *"},
			&controllerv1alpha1.BackupCronJobConfig{Schedule: "1 * * * *"},
			true,
		),
		Entry("All fields match => no change",
			&controllerv1alpha1.BackupCronJobConfig{
				Enable:   pointer.Bool(true),
				Schedule: "0 * * * *",
			},
			&controllerv1alpha1.BackupCronJobConfig{
				Enable:   pointer.Bool(true),
				Schedule: "0 * * * *",
			},
			false,
		),
	)
})
