//
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
//

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"

	"github.com/operator-framework/operator-lib/prune"
	"github.com/robfig/cron/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DevWorkspacePrunerReconciler reconciles DevWorkspace objects for pruning purposes.
type DevWorkspacePrunerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	cron *cron.Cron
}

func shouldReconcileOnUpdate(e event.UpdateEvent, log logr.Logger) bool {
	log.Info("DevWorkspaceOperatorConfig update event received")
	oldConfig, ok := e.ObjectOld.(*controllerv1alpha1.DevWorkspaceOperatorConfig)
	if !ok {
		return false
	}
	newConfig, ok := e.ObjectNew.(*controllerv1alpha1.DevWorkspaceOperatorConfig)
	if !ok {
		return false
	}

	oldCleanup := oldConfig.Config.Workspace.CleanupCronJob
	newCleanup := newConfig.Config.Workspace.CleanupCronJob

	differentBool := func(a, b *bool) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil || b == nil:
			return true
		default:
			return *a != *b
		}
	}
	differentInt32 := func(a, b *int32) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil || b == nil:
			return true
		default:
			return *a != *b
		}
	}

	if oldCleanup == nil && newCleanup == nil {
		return false
	}
	if (oldCleanup == nil && newCleanup != nil) || (oldCleanup != nil && newCleanup == nil) {
		return true
	}
	if differentBool(oldCleanup.Enable, newCleanup.Enable) {
		return true
	}
	if differentBool(oldCleanup.DryRun, newCleanup.DryRun) {
		return true
	}
	if differentInt32(oldCleanup.RetainTime, newCleanup.RetainTime) {
		return true
	}
	return oldCleanup.Schedule != newCleanup.Schedule
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevWorkspacePrunerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log := r.Log.WithName("setupWithManager")
	log.Info("Setting up DevWorkspacePrunerReconciler")

	configPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldReconcileOnUpdate(e, log)
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	maxConcurrentReconciles, err := config.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	// Initialize cron scheduler
	r.cron = cron.New()

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&controllerv1alpha1.DevWorkspaceOperatorConfig{}).
		WithEventFilter(configPredicate).
		Complete(r)
}

// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces,verbs=get;list;delete
// +kubebuilder:rbac:groups=controller.devfile.io,resources=devworkspaceoperatorconfigs,verbs=get;list;watch

// Reconcile is the main reconciliation loop for the pruner controller.
func (r *DevWorkspacePrunerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log
	log.Info("Reconciling DevWorkspacePruner", "DWOC", req.NamespacedName)

	dwOperatorConfig := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
	err := r.Get(ctx, req.NamespacedName, dwOperatorConfig)
	if err != nil {
		log.Error(err, "Failed to get DevWorkspaceOperatorConfig")
		r.stopCron(log)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cleanupConfig := dwOperatorConfig.Config.Workspace.CleanupCronJob
	log = log.WithValues("CleanupCronJob", cleanupConfig)

	if cleanupConfig == nil {
		log.Info("DevWorkspaceOperatorConfig does not have cleanup configuration, stopping cron schedler and skipping reconciliation")
		r.stopCron(log)
		return ctrl.Result{}, nil
	}
	if cleanupConfig.Enable == nil || !*cleanupConfig.Enable {
		log.Info("DevWorkspace pruning is disabled, stopping cron scheduler and skipping reconciliation")
		r.stopCron(log)
		return ctrl.Result{}, nil
	}
	if cleanupConfig.Schedule == "" {
		log.Info("DevWorkspace pruning schedule is not defined, stopping cron scheduler and skipping reconciliation")
		r.stopCron(log)
		return ctrl.Result{}, nil
	}

	r.startCron(ctx, cleanupConfig, log)

	return ctrl.Result{}, nil
}

func (r *DevWorkspacePrunerReconciler) startCron(ctx context.Context, cleanupConfig *controllerv1alpha1.CleanupCronJobConfig, logger logr.Logger) {
	log := logger.WithName("cron")
	log.Info("Starting cron scheduler")

	// remove existing cronjob tasks
	// we cannot update the existing tasks, so we need to remove them and add new ones
	entries := r.cron.Entries()
	for _, entry := range entries {
		r.cron.Remove(entry.ID)
	}

	// add cronjob task
	_, err := r.cron.AddFunc(cleanupConfig.Schedule, func() {
		taskLog := logger.WithName("cronTask")

		// define pruning parameters
		retainTime := time.Duration(*cleanupConfig.RetainTime) * time.Second

		dryRun := false
		if cleanupConfig.DryRun != nil {
			dryRun = *cleanupConfig.DryRun
		}

		taskLog.Info("Starting DevWorkspace pruning job")
		if err := r.pruneDevWorkspaces(ctx, retainTime, dryRun, logger); err != nil {
			taskLog.Error(err, "Failed to prune DevWorkspaces")
		}
		taskLog.Info("DevWorkspace pruning job finished")
	})
	if err != nil {
		log.Error(err, "Failed to add cronjob function")
		return
	}

	r.cron.Start()
}

func (r *DevWorkspacePrunerReconciler) stopCron(logger logr.Logger) {
	log := logger.WithName("cron")
	log.Info("Stopping cron scheduler")

	// remove existing cronjob tasks
	entries := r.cron.Entries()
	for _, entry := range entries {
		r.cron.Remove(entry.ID)
	}

	ctx := r.cron.Stop()
	ctx.Done()

	log.Info("Cron scheduler stopped")
}

func (r *DevWorkspacePrunerReconciler) pruneDevWorkspaces(ctx context.Context, retainTime time.Duration, dryRun bool, logger logr.Logger) error {
	log := logger.WithName("pruner")

	// create a prune strategy based on the configuration
	var pruneStrategy prune.StrategyFunc
	if dryRun {
		pruneStrategy = r.dryRunPruneStrategy(retainTime, log)
	} else {
		pruneStrategy = r.pruneStrategy(retainTime, log)
	}

	gvk := schema.GroupVersionKind{
		Group:   dwv2.SchemeGroupVersion.Group,
		Version: dwv2.SchemeGroupVersion.Version,
		Kind:    "DevWorkspace",
	}

	// create pruner that uses our custom strategy
	pruner, err := prune.NewPruner(r.Client, gvk, pruneStrategy)
	if err != nil {
		return fmt.Errorf("failed to create pruner: %w", err)
	}

	deletedObjects, err := pruner.Prune(ctx)
	if err != nil {
		return fmt.Errorf("failed to prune objects: %w", err)
	}
	log.Info(fmt.Sprintf("Pruned %d DevWorkspaces", len(deletedObjects)))

	for _, obj := range deletedObjects {
		devWorkspace, ok := obj.(*dwv2.DevWorkspace)
		if !ok {
			log.Error(err, fmt.Sprintf("failed to convert %v to DevWorkspace", obj))
			continue
		}
		log.Info(fmt.Sprintf("Pruned DevWorkspace '%s' in namespace '%s'", devWorkspace.Name, devWorkspace.Namespace))
	}

	return nil
}

// pruneStrategy returns a StrategyFunc that will return a list of
// DevWorkspaces to prune based on the lastTransitionTime of the 'Started' condition.
func (r *DevWorkspacePrunerReconciler) pruneStrategy(retainTime time.Duration, logger logr.Logger) prune.StrategyFunc {
	log := logger.WithName("pruneStrategy")

	return func(ctx context.Context, objs []client.Object) ([]client.Object, error) {
		filteredObjs := filterByInactivityTime(objs, retainTime, log)
		log.Info(fmt.Sprintf("Found %d DevWorkspaces to prune", len(filteredObjs)))
		return filteredObjs, nil
	}
}

// dryRunPruneStrategy returns a StrategyFunc that will always return an empty list of DevWorkspaces to prune.
// This is used for dry-run mode.
func (r *DevWorkspacePrunerReconciler) dryRunPruneStrategy(retainTime time.Duration, logger logr.Logger) prune.StrategyFunc {
	log := logger.WithName("dryRunPruneStrategy")

	return func(ctx context.Context, objs []client.Object) ([]client.Object, error) {
		filteredObjs := filterByInactivityTime(objs, retainTime, log)
		log.Info(fmt.Sprintf("Found %d DevWorkspaces to prune", len(filteredObjs)))

		// Return an empty list of DevWorkspaces because this is a dry-run
		log.Info("Dry run mode: no DevWorkspaces will be pruned")
		return []client.Object{}, nil
	}
}

// filterByInactivityTime filters DevWorkspaces based on the lastTransitionTime of the 'Started' condition.
func filterByInactivityTime(objs []client.Object, retainTime time.Duration, log logr.Logger) []client.Object {
	var filteredObjs []client.Object
	for _, obj := range objs {
		devWorkspace, ok := obj.(*dwv2.DevWorkspace)
		if !ok {
			log.Error(nil, fmt.Sprintf("failed to convert %v to DevWorkspace", obj))
			continue
		}

		if canPrune(*devWorkspace, retainTime, log) {
			filteredObjs = append(filteredObjs, devWorkspace)
			log.Info(fmt.Sprintf("Adding DevWorkspace '%s/%s' to prune list", devWorkspace.Namespace, devWorkspace.Name))
		} else {
			log.Info(fmt.Sprintf("Skipping DevWorkspace '%s/%s': not eligible for pruning", devWorkspace.Namespace, devWorkspace.Name))
		}
	}
	return filteredObjs
}

// canPrune returns true if the DevWorkspace is eligible for pruning.
func canPrune(dw dwv2.DevWorkspace, retainTime time.Duration, log logr.Logger) bool {
	// Skip started and running DevWorkspaces
	if dw.Spec.Started {
		log.Info(fmt.Sprintf("Skipping DevWorkspace '%s/%s': already started", dw.Namespace, dw.Name))
		return false
	}

	var startTime *metav1.Time
	startedCondition := conditions.GetConditionByType(dw.Status.Conditions, conditions.Started)
	if startedCondition != nil {
		startTime = &startedCondition.LastTransitionTime
	}
	if startTime == nil {
		log.Info(fmt.Sprintf("Skipping DevWorkspace '%s/%s': missing 'Started' condition", dw.Namespace, dw.Name))
		return false
	}
	if time.Since(startTime.Time) <= retainTime {
		log.Info(fmt.Sprintf("Skipping DevWorkspace '%s/%s': not eligible for pruning", dw.Namespace, dw.Name))
		return false
	}
	return true
}
