//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"

	"github.com/devfile/devworkspace-operator/pkg/library/flatten"

	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	shimlib "github.com/devfile/devworkspace-operator/pkg/library/shim"
	storagelib "github.com/devfile/devworkspace-operator/pkg/library/storage"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/timing"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	devworkspace "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

type currentStatus struct {
	// Map of condition types that are true for the current workspace. Key is valid condition, value is optional
	// message to be filled into condition's 'Message' field.
	Conditions map[devworkspace.WorkspaceConditionType]string
	// Current workspace phase
	Phase devworkspace.WorkspacePhase
}

// DevWorkspaceReconciler reconciles a DevWorkspace object
type DevWorkspaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

/////// CRD-related RBAC roles
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=controller.devfile.io,resources=*,verbs=*
/////// Required permissions for controller
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments;replicasets,verbs=*
// +kubebuilder:rbac:groups="",resources=pods;serviceaccounts;secrets;configmaps;persistentvolumeclaims,verbs=*
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;create;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=oauth.openshift.io,resources=oauthclients,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups=apps,resourceNames=devworkspace-controller,resources=deployments/finalizers,verbs=update
/////// Required permissions for workspace ServiceAccount
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=apps;extensions,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments,verbs=get;list;watch

func (r *DevWorkspaceReconciler) Reconcile(req ctrl.Request) (reconcileResult ctrl.Result, err error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	clusterAPI := provision.ClusterAPI{
		Client: r.Client,
		Scheme: r.Scheme,
		Logger: reqLogger,
	}

	// Fetch the Workspace instance
	workspace := &devworkspace.DevWorkspace{}
	err = r.Get(ctx, req.NamespacedName, workspace)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	reqLogger = reqLogger.WithValues(config.WorkspaceIDLoggerKey, workspace.Status.WorkspaceId)
	reqLogger.Info("Reconciling Workspace")

	// Check if the WorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if workspace.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspace")
		return r.finalize(ctx, reqLogger, workspace)
	}

	// Ensure workspaceID is set.
	if workspace.Status.WorkspaceId == "" {
		workspaceId, err := getWorkspaceId(workspace)
		if err != nil {
			return reconcile.Result{}, err
		}
		workspace.Status.WorkspaceId = workspaceId
		err = r.Status().Update(ctx, workspace)
		return reconcile.Result{Requeue: true}, err
	}

	// Handle stopped workspaces
	if !workspace.Spec.Started {
		timing.ClearAnnotations(workspace)
		r.syncTimingToCluster(ctx, workspace, map[string]string{}, reqLogger)
		return r.stopWorkspace(workspace, reqLogger)
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{
		Conditions: map[devworkspace.WorkspaceConditionType]string{},
		Phase:      devworkspace.WorkspaceStatusStarting,
	}
	clusterWorkspace := workspace.DeepCopy()
	timingInfo := map[string]string{}
	timing.SetTime(timingInfo, timing.WorkspaceStarted)
	// Defer function to make sure workspace status is up to date based on where we finish the reconcile.
	// Within the function, reconcileResult and err are set to whatever is returned from Reconcile()
	defer func() (reconcile.Result, error) {
		r.syncTimingToCluster(ctx, clusterWorkspace, timingInfo, reqLogger)
		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	webhooks, msg, err := r.validateWebhooksConfig()
	if err != nil {
		return r.killWorkspace(workspace, msg, &reconcileStatus)
	}
	if workspace.Annotations[config.WorkspaceRestrictedAccessAnnotation] == "true" {
		if workspace.CreationTimestamp.Before(&webhooks.validating.CreationTimestamp) ||
			workspace.CreationTimestamp.Before(&webhooks.mutating.CreationTimestamp) {
			msg := "DevWorkspace was created before current webhooks were installed and must be recreated to successfully start"
			return r.killWorkspace(workspace, msg, &reconcileStatus)
		}
		if _, present := workspace.Labels[config.WorkspaceCreatorLabel]; !present {
			msg := "DevWorkspace was created without creator ID label. It must be recreated to resolve the issue"
			return r.killWorkspace(workspace, msg, &reconcileStatus)
		}
	}

	if _, ok := clusterWorkspace.Annotations[config.WorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, config.WorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), clusterWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	timing.SetTime(timingInfo, timing.ComponentsCreated)
	// TODO#185 : Temporarily do devfile flattening in main reconcile loop; this should be moved to a subcontroller.
	flattenHelpers := flatten.ResolverTools{
		InstanceNamespace: workspace.Namespace,
		Context:           ctx,
		K8sClient:         r.Client,
		InternalRegistry:  &registry.InternalRegistryImpl{},
		HttpClient:        http.DefaultClient,
	}
	flattenedWorkspace, err := flatten.ResolveDevWorkspace(workspace.Spec.Template, flattenHelpers)
	if err != nil {
		return r.failWorkspace(fmt.Sprintf("Error processing devfile: %s", err), reqLogger, &reconcileStatus)
	}
	workspace.Spec.Template = *flattenedWorkspace
	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if isFinalizerNecessary(workspace) {
		if ok, err := r.setFinalizer(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		} else if !ok {
			return reconcile.Result{Requeue: true}, nil
		}
	}

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(workspace.Spec.Template)
	if err != nil {
		return r.failWorkspace(fmt.Sprintf("Error processing devfile: %s", err), reqLogger, &reconcileStatus)
	}
	err = storagelib.RewriteContainerVolumeMounts(workspace.Status.WorkspaceId, devfilePodAdditions, workspace.Spec.Template)
	if err != nil {
		return r.failWorkspace(fmt.Sprintf("Error processing devfile volumes: %s", err), reqLogger, &reconcileStatus)
	}
	shimlib.FillDefaultEnvVars(devfilePodAdditions, *workspace)
	allPodAdditions := []controllerv1alpha1.PodAdditions{*devfilePodAdditions}
	reconcileStatus.Conditions[devworkspace.WorkspaceComponentsReady] = ""
	timing.SetTime(timingInfo, timing.ComponentsReady)

	if storagelib.NeedsStorage(workspace.Spec.Template) {
		pvcStatus := provision.SyncPVC(workspace, r.Client, reqLogger)
		if pvcStatus.Err != nil || !pvcStatus.Continue {
			return reconcile.Result{Requeue: true}, pvcStatus.Err
		}
	}

	rbacStatus := provision.SyncRBAC(workspace, r.Client, reqLogger)
	if rbacStatus.Err != nil || !rbacStatus.Continue {
		return reconcile.Result{Requeue: true}, rbacStatus.Err
	}

	// Step two: Create routing, and wait for routing to be ready
	timing.SetTime(timingInfo, timing.RoutingCreated)
	routingStatus := provision.SyncRoutingToCluster(workspace, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			return r.failWorkspace("Failed to install network objects required for devworkspace", reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on routing to be ready")
		return reconcile.Result{Requeue: routingStatus.Requeue}, routingStatus.Err
	}
	reconcileStatus.Conditions[devworkspace.WorkspaceRoutingReady] = ""
	timing.SetTime(timingInfo, timing.RoutingReady)

	statusOk, err := syncWorkspaceIdeURL(clusterWorkspace, routingStatus.ExposedEndpoints, clusterAPI)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !statusOk {
		reqLogger.Info("Updating workspace status")
		return reconcile.Result{Requeue: true}, nil
	}

	// Step four: Collect all workspace deployment contributions
	routingPodAdditions := routingStatus.PodAdditions
	if routingPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *routingPodAdditions)
	}

	// Step five: Prepare workspace ServiceAccount
	saAnnotations := map[string]string{}
	if routingPodAdditions != nil {
		saAnnotations = routingPodAdditions.ServiceAccountAnnotations
	}
	serviceAcctStatus := provision.SyncServiceAccount(workspace, saAnnotations, clusterAPI)
	if !serviceAcctStatus.Continue {
		// FailStartup is not possible for generating the serviceaccount
		reqLogger.Info("Waiting for workspace ServiceAccount")
		return reconcile.Result{Requeue: serviceAcctStatus.Requeue}, serviceAcctStatus.Err
	}
	serviceAcctName := serviceAcctStatus.ServiceAccountName
	reconcileStatus.Conditions[devworkspace.WorkspaceServiceAccountReady] = ""

	// Step six: Create deployment and wait for it to be ready
	timing.SetTime(timingInfo, timing.DeploymentCreated)
	deploymentStatus := provision.SyncDeploymentToCluster(workspace, allPodAdditions, serviceAcctName, clusterAPI)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			return r.failWorkspace(fmt.Sprintf("Devworkspace spec is invalid: %s", deploymentStatus.Err), reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on deployment to be ready")
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.Conditions[devworkspace.WorkspaceReady] = ""
	timing.SetTime(timingInfo, timing.DeploymentReady)

	serverReady, err := checkServerStatus(clusterWorkspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	timing.SetTime(timingInfo, timing.WorkspaceReady)
	timing.SummarizeStartup(clusterWorkspace)
	reconcileStatus.Phase = devworkspace.WorkspaceStatusRunning
	return reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) stopWorkspace(workspace *devworkspace.DevWorkspace, logger logr.Logger) (reconcile.Result, error) {
	workspaceDeployment := &appsv1.Deployment{}
	namespaceName := types.NamespacedName{
		Name:      common.DeploymentName(workspace.Status.WorkspaceId),
		Namespace: workspace.Namespace,
	}
	status := &currentStatus{}
	err := r.Get(context.TODO(), namespaceName, workspaceDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			status.Phase = devworkspace.WorkspaceStatusStopped
			return r.updateWorkspaceStatus(workspace, logger, status, reconcile.Result{}, nil)
		}
		return reconcile.Result{}, err
	}

	status.Phase = devworkspace.WorkspaceStatusStopping
	replicas := workspaceDeployment.Spec.Replicas
	if replicas == nil || *replicas > 0 {
		logger.Info("Stopping workspace")
		patch := client.MergeFrom(workspaceDeployment.DeepCopy())
		var replicasZero int32 = 0
		workspaceDeployment.Spec.Replicas = &replicasZero
		err = r.Patch(context.TODO(), workspaceDeployment, patch)
		if err != nil && !k8sErrors.IsConflict(err) {
			return reconcile.Result{}, err
		}
		return r.updateWorkspaceStatus(workspace, logger, status, reconcile.Result{}, nil)
	}

	if workspaceDeployment.Status.Replicas == 0 {
		logger.Info("Workspace stopped")
		status.Phase = devworkspace.WorkspaceStatusStopped
	}
	return r.updateWorkspaceStatus(workspace, logger, status, reconcile.Result{}, nil)
}

func (r *DevWorkspaceReconciler) syncTimingToCluster(
	ctx context.Context, workspace *devworkspace.DevWorkspace, timingInfo map[string]string, reqLogger logr.Logger) {
	if timing.IsEnabled() {
		if workspace.Annotations == nil {
			workspace.Annotations = map[string]string{}
		}
		for timingEvent, timestamp := range timingInfo {
			if _, set := workspace.Annotations[timingEvent]; !set {
				workspace.Annotations[timingEvent] = timestamp
			}
		}
		if err := r.Update(ctx, workspace); err != nil {
			if k8sErrors.IsConflict(err) {
				reqLogger.Info("Got conflict when trying to apply timing annotations to workspace")
			} else {
				reqLogger.Error(err, "Error trying to apply timing annotations to devworkspace")
			}
		}
	}
}

func getWorkspaceId(instance *devworkspace.DevWorkspace) (string, error) {
	uid, err := uuid.Parse(string(instance.UID))
	if err != nil {
		return "", err
	}
	return "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""), nil
}

// killWorkspace deletes a workspace's deployment and then sets its status to failed, using the message provided as the
// condition message. Changes to the DevWorkspace object are not propagated to the cluster and must be synced via
// updateWorkspaceStatus()
func (r *DevWorkspaceReconciler) killWorkspace(workspace *devworkspace.DevWorkspace, msg string, status *currentStatus) (reconcile.Result, error) {
	wait, err := provision.DeleteWorkspaceDeployment(context.Background(), workspace, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if wait {
		return reconcile.Result{Requeue: true}, nil
	}
	status.Phase = devworkspace.WorkspaceStatusFailed
	status.Conditions[devworkspace.WorkspaceFailedStart] = msg
	return reconcile.Result{}, nil
}

// failWorkspace marks a workspace as failed by setting relevant fields in the status struct. These changes are not
// propagated to the cluster, and must be synced via updateWorkspaceStatus()
func (r *DevWorkspaceReconciler) failWorkspace(msg string, logger logr.Logger, status *currentStatus) (reconcile.Result, error) {
	logger.Info("Workspace start failed")
	status.Phase = devworkspace.WorkspaceStatusFailed
	status.Conditions[devworkspace.WorkspaceFailedStart] = msg
	return reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Set up indexing https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#setup
	return ctrl.NewControllerManagedBy(mgr).
		For(&devworkspace.DevWorkspace{}).
		// List DevWorkspaceTemplates as owned to enable updating workspaces when templates
		// are changed; this should be moved to whichever controller is responsible for flattening
		// DevWorkspaces
		Owns(&devworkspace.DevWorkspaceTemplate{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Owns(&controllerv1alpha1.Component{}).
		Owns(&controllerv1alpha1.WorkspaceRouting{}).
		WithEventFilter(predicates).
		Complete(r)
}
