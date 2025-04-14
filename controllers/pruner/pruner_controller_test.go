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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron/v3"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("DevWorkspacePrunerReconciler", func() {
	var (
		ctx           context.Context
		fakeClient    client.Client
		reconciler    DevWorkspacePrunerReconciler
		nameNamespace types.NamespacedName
		log           logr.Logger
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(controllerv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(dwv2.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		log = zap.New(zap.UseDevMode(true)).WithName("prunerController")

		reconciler = DevWorkspacePrunerReconciler{
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

	Context("Helper Functions", func() {
		var (
			retainTime    time.Duration
			dw1, dw2, dw3 dwv2.DevWorkspace
		)

		BeforeEach(func() {
			retainTime = 1 * time.Minute

			// Create DevWorkspaces
			// dw1 is running
			dw1 = *createDevWorkspace("dw1", "test-ns", true, metav1.Now())
			// dw2 is inactive for 2 minutes
			dw2 = *createDevWorkspace("dw2", "test-ns", false, metav1.NewTime(time.Now().Add(-2*time.Minute)))
			// dw3 was recently active
			dw3 = *createDevWorkspace("dw3", "test-ns", false, metav1.NewTime(time.Now().Add(-30*time.Second)))
		})

		Describe("canPrune", func() {
			It("Should return false if DevWorkspace is started", func() {
				result := canPrune(dw1, retainTime, log)
				Expect(result).To(BeFalse())
			})

			It("Should return false if 'Started' condition is missing", func() {
				dw4 := dwv2.DevWorkspace{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dw4",
						Namespace: "test-ns",
					},
					Spec: dwv2.DevWorkspaceSpec{
						Started: false,
					},
					Status: dwv2.DevWorkspaceStatus{
						Conditions: []dwv2.DevWorkspaceCondition{}, // Empty conditions
					},
				}
				result := canPrune(dw4, retainTime, log)
				Expect(result).To(BeFalse())
			})

			It("Should return false if DevWorkspace was recently active", func() {
				result := canPrune(dw3, retainTime, log)
				Expect(result).To(BeFalse())
			})

			It("Should return true if DevWorkspace is inactive", func() {
				result := canPrune(dw2, retainTime, log)
				Expect(result).To(BeTrue())
			})
		})

		Describe("filterByInactivityTime", func() {
			It("Should return only inactive DevWorkspaces", func() {
				objs := []client.Object{
					&dw1,
					&dw2,
					&dw3,
				}
				filteredObjs := filterByInactivityTime(objs, retainTime, log)
				Expect(filteredObjs).To(HaveLen(1))
				Expect(filteredObjs[0].GetName()).To(Equal("dw2"))
			})
		})
	})

	Context("Reconcile", func() {
		It("Should do nothing if DevWorkspaceOperatorConfig is not found", func() {
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
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
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

		It("Should not start cron if CleanupCronJob is nil", func() {
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should do not start cron if pruning is disabled", func() {
			enabled := false
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable: &enabled,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should do not start cron if schedule is missing", func() {
			enabled := true
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable:   &enabled,
							Schedule: "",
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: nameNamespace})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(reconciler.cron.Entries()).To(BeEmpty())
		})

		It("Should start cron if pruning is enabled and schedule is defined", func() {
			enabled := true
			schedule := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
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
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
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
			dwoc.Config.Workspace.CleanupCronJob.Schedule = schedule2
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
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
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

	Context("Prune DevWorkspaces", func() {
		var (
			retainTime    time.Duration
			dryRun        bool
			dw1, dw2, dw3 *dwv2.DevWorkspace
		)

		BeforeEach(func() {
			// Set up test parameters
			retainTime = 1 * time.Minute
			dryRun = false

			// Create DevWorkspaces
			// dw1 is running
			dw1 = createDevWorkspace("dw1", "test-ns", true, metav1.Now())
			// dw2 is inactive for 2 minutes
			dw2 = createDevWorkspace("dw2", "test-ns", false, metav1.NewTime(time.Now().Add(-2*time.Minute)))
			// dw3 was recently active
			dw3 = createDevWorkspace("dw3", "test-ns", false, metav1.NewTime(time.Now().Add(-30*time.Second)))

			Expect(fakeClient.Create(ctx, dw1)).To(Succeed())
			Expect(fakeClient.Create(ctx, dw2)).To(Succeed())
			Expect(fakeClient.Create(ctx, dw3)).To(Succeed())
		})

		AfterEach(func() {
			Expect(fakeClient.Delete(ctx, dw1)).To(Succeed())
			// Check if dw2 exists before deleting
			if err := fakeClient.Get(ctx, types.NamespacedName{Name: "dw2", Namespace: "test-ns"}, &dwv2.DevWorkspace{}); err == nil {
				Expect(fakeClient.Delete(ctx, dw2)).To(Succeed())
			}
			Expect(fakeClient.Delete(ctx, dw3)).To(Succeed())
		})

		It("Should prune inactive DevWorkspaces", func() {
			err := reconciler.pruneDevWorkspaces(ctx, retainTime, dryRun, log)
			Expect(err).ToNot(HaveOccurred())

			// Check if dw2 is deleted
			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw2", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			// Check if dw1 and dw3 still exist
			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw1", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw3", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should not prune any DevWorkspaces in dryRun mode", func() {
			dryRun := true
			err := reconciler.pruneDevWorkspaces(ctx, retainTime, dryRun, log)
			Expect(err).ToNot(HaveOccurred())

			// Check that all DevWorkspaces still exist
			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw1", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw2", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).ToNot(HaveOccurred())

			err = fakeClient.Get(ctx, types.NamespacedName{Name: "dw3", Namespace: "test-ns"}, &dwv2.DevWorkspace{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Cron management functions", func() {
		It("Should start cron", func() {
			enabled := true
			schedule := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable:   &enabled,
							Schedule: schedule,
						},
					},
				},
			}

			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())
			Expect(reconciler.cron.Entries()).To(BeEmpty())

			reconciler.startCron(ctx, dwoc.Config.Workspace.CleanupCronJob, log)

			Expect(reconciler.cron.Entries()).To(HaveLen(1))
		})

		It("Should stop cron", func() {
			enabled := true
			schedule := "* * * * *"
			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nameNamespace.Name, Namespace: nameNamespace.Namespace},
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: &controllerv1alpha1.CleanupCronJobConfig{
							Enable:   &enabled,
							Schedule: schedule,
						},
					},
				},
			}

			Expect(fakeClient.Create(ctx, dwoc)).To(Succeed())
			Expect(reconciler.cron.Entries()).To(BeEmpty())

			reconciler.startCron(ctx, dwoc.Config.Workspace.CleanupCronJob, log)

			Expect(reconciler.cron.Entries()).To(HaveLen(1))

			reconciler.stopCron(log)

			Expect(reconciler.cron.Entries()).To(BeEmpty())
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

	DescribeTable("Testing UpdateFunc for cleanup configuration changes",
		func(oldCleanup, newCleanup *controllerv1alpha1.CleanupCronJobConfig, expected bool) {
			oldCfg := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: oldCleanup,
					},
				},
			}
			newCfg := &controllerv1alpha1.DevWorkspaceOperatorConfig{
				Config: &controllerv1alpha1.OperatorConfiguration{
					Workspace: &controllerv1alpha1.WorkspaceConfig{
						CleanupCronJob: newCleanup,
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
		Entry("OldCleanup==nil, NewCleanup!=nil => changed", nil, &controllerv1alpha1.CleanupCronJobConfig{}, true),
		Entry("OldCleanup!=nil, NewCleanup==nil => changed", &controllerv1alpha1.CleanupCronJobConfig{}, nil, true),
		Entry("OldCleanup.Enable==nil, NewCleanup.Enable==nil => no change",
			&controllerv1alpha1.CleanupCronJobConfig{Enable: nil},
			&controllerv1alpha1.CleanupCronJobConfig{Enable: nil},
			false,
		),
		Entry("OldCleanup.Enable==nil, NewCleanup.Enable!=nil => changed",
			&controllerv1alpha1.CleanupCronJobConfig{Enable: nil},
			&controllerv1alpha1.CleanupCronJobConfig{Enable: pointer.Bool(true)},
			true,
		),
		Entry("OldCleanup.Enable!=nil, NewCleanup.Enable==nil => changed",
			&controllerv1alpha1.CleanupCronJobConfig{Enable: pointer.Bool(true)},
			&controllerv1alpha1.CleanupCronJobConfig{Enable: nil},
			true,
		),
		Entry("Enable differs => changed",
			&controllerv1alpha1.CleanupCronJobConfig{Enable: pointer.Bool(true)},
			&controllerv1alpha1.CleanupCronJobConfig{Enable: pointer.Bool(false)},
			true,
		),
		Entry("OldCleanup.DryRun==nil, NewCleanup.DryRun==nil => no change",
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: nil},
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: nil},
			false,
		),
		Entry("OldCleanup.DryRun==nil, NewCleanup.DryRun!=nil => changed",
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: nil},
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: pointer.Bool(true)},
			true,
		),
		Entry("OldCleanup.DryRun!=nil, NewCleanup.DryRun==nil => changed",
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: pointer.Bool(true)},
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: nil},
			true,
		),
		Entry("DryRun differs => changed",
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: pointer.Bool(true)},
			&controllerv1alpha1.CleanupCronJobConfig{DryRun: pointer.Bool(false)},
			true,
		),
		Entry("RetainTime differs => changed",
			&controllerv1alpha1.CleanupCronJobConfig{RetainTime: pointer.Int32(1)},
			&controllerv1alpha1.CleanupCronJobConfig{RetainTime: pointer.Int32(2)},
			true,
		),
		Entry("Schedule differs => changed",
			&controllerv1alpha1.CleanupCronJobConfig{Schedule: "0 * * * *"},
			&controllerv1alpha1.CleanupCronJobConfig{Schedule: "1 * * * *"},
			true,
		),
		Entry("All fields match => no change",
			&controllerv1alpha1.CleanupCronJobConfig{
				Enable:     pointer.Bool(true),
				DryRun:     pointer.Bool(false),
				RetainTime: pointer.Int32(5),
				Schedule:   "0 * * * *",
			},
			&controllerv1alpha1.CleanupCronJobConfig{
				Enable:     pointer.Bool(true),
				DryRun:     pointer.Bool(false),
				RetainTime: pointer.Int32(5),
				Schedule:   "0 * * * *",
			},
			false,
		),
	)
})
