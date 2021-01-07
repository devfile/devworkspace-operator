//
// Copyright (c) 2019-2020 Red Hat, Inc.
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
	"errors"
	"fmt"
	"strings"
	"time"

	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	shimlib "github.com/devfile/devworkspace-operator/pkg/library/shim"
	storagelib "github.com/devfile/devworkspace-operator/pkg/library/storage"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/controllers/workspace/restapis"
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

	devworkspace "github.com/devfile/api/pkg/apis/workspaces/v1alpha2"
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
		r.syncTimingToCluster(ctx, workspace, reqLogger)
		return r.stopWorkspace(workspace, reqLogger)
	}

	// Set finalizer on DevWorkspace if necessary
	if ok, err := r.setFinalizer(ctx, workspace); err != nil {
		return reconcile.Result{}, err
	} else if !ok {
		return reconcile.Result{Requeue: true}, nil
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{
		Conditions: map[devworkspace.WorkspaceConditionType]string{},
		Phase:      devworkspace.WorkspaceStatusStarting,
	}
	timing.SetTime(workspace, timing.WorkspaceStarted)
	defer func() (reconcile.Result, error) {
		r.syncTimingToCluster(ctx, workspace, reqLogger)
		return r.updateWorkspaceStatus(workspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	msg, err := r.validateCreatorTimestamp(workspace)
	if err != nil {
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = msg
		return reconcile.Result{}, err
	}

	_, ok := workspace.Annotations[config.WorkspaceStopReasonAnnotation]
	if ok {
		delete(workspace.Annotations, config.WorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), workspace)
		return reconcile.Result{Requeue: true}, err
	}

	restrictedAccess := workspace.Annotations[config.WorkspaceRestrictedAccessAnnotation]
	if restrictedAccess == "true" && config.ControllerCfg.GetWebhooksEnabled() != "true" {
		reqLogger.Info("Workspace is configured to have restricted access but webhooks are not enabled.")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = "Workspace has restricted-access annotation " +
			"applied but operator does not have webhooks enabled. " +
			"Remove restricted-access annotation or ask an administrator " +
			"to reconfigure Operator."
		return reconcile.Result{}, nil
	}

	timing.SetTime(workspace, timing.ComponentsCreated)
	// TODO#185 : Move away from using devfile 1.0 constructs; only work on flattened devfiles until
	// TODO#185 : plugins is figured out.
	// TODO#185 : Implement defaulting container component for Web Terminals for compatibility
	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(workspace.Spec.Template)
	if err != nil {
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error processing devfile: %s", err)
		return reconcile.Result{}, nil
	}
	err = storagelib.RewriteContainerVolumeMounts(workspace.Status.WorkspaceId, devfilePodAdditions, workspace.Spec.Template)
	if err != nil {
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error processing devfile volumes: %s", err)
		return reconcile.Result{}, nil
	}
	componentDescriptions, err := shimlib.GetComponentDescriptionsFromPodAdditions(devfilePodAdditions, workspace.Spec.Template)
	if err != nil {
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error processing devfile for Theia: %s", err)
		return reconcile.Result{}, nil
	}
	reconcileStatus.Conditions[devworkspace.WorkspaceReady] = ""
	timing.SetTime(workspace, timing.ComponentsReady)

	// Only add che rest apis if Theia editor is present in the devfile
	if restapis.IsCheRestApisRequired(workspace.Spec.Template.Components) {
		if !restapis.IsCheRestApisConfigured() {
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = "Che REST API sidecar is not configured but required for the Theia plugin"
			return reconcile.Result{Requeue: false}, errors.New("che REST API sidecar is not configured but required for used Theia plugin")
		}
		// TODO: first half of provisioning rest-apis
		cheRestApisComponent := restapis.GetCheRestApisComponent(workspace.Name, workspace.Status.WorkspaceId, workspace.Namespace)
		// Some containers (e.g. Theia) need Che API Sidecar to be available just after start up. So, putting Che API Sidecar first before all
		componentDescriptions = append([]controllerv1alpha1.ComponentDescription{cheRestApisComponent}, componentDescriptions...)
	}

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
	timing.SetTime(workspace, timing.RoutingCreated)
	routingStatus := provision.SyncRoutingToCluster(workspace, componentDescriptions, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			reqLogger.Info("DevWorkspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			// TODO: Propagate failure reason from workspaceRouting
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = "Failed to install network objects required for devworkspace"
			return reconcile.Result{}, routingStatus.Err
		}
		reqLogger.Info("Waiting on routing to be ready")
		return reconcile.Result{Requeue: routingStatus.Requeue}, routingStatus.Err
	}
	reconcileStatus.Conditions[devworkspace.WorkspaceRoutingReady] = ""
	timing.SetTime(workspace, timing.RoutingReady)

	statusOk, err := syncWorkspaceIdeURL(workspace, routingStatus.ExposedEndpoints, clusterAPI)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !statusOk {
		reqLogger.Info("Updating workspace status")
		return reconcile.Result{Requeue: true}, nil
	}

	// Step three: setup che-rest-apis configmap
	if restapis.IsCheRestApisRequired(workspace.Spec.Template.Components) {
		configMapStatus := restapis.SyncRestAPIsConfigMap(workspace, componentDescriptions, routingStatus.ExposedEndpoints, clusterAPI)
		if !configMapStatus.Continue {
			// FailStartup is not possible for generating the configmap
			reqLogger.Info("Waiting on che-rest-apis configmap to be ready")
			return reconcile.Result{Requeue: configMapStatus.Requeue}, configMapStatus.Err
		}
	}

	// Step four: Collect all workspace deployment contributions
	routingPodAdditions := routingStatus.PodAdditions
	var podAdditions []controllerv1alpha1.PodAdditions
	for _, component := range componentDescriptions {
		podAdditions = append(podAdditions, component.PodAdditions)
	}
	if routingPodAdditions != nil {
		podAdditions = append(podAdditions, *routingPodAdditions)
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
	timing.SetTime(workspace, timing.DeploymentCreated)
	deploymentStatus := provision.SyncDeploymentToCluster(workspace, podAdditions, serviceAcctName, clusterAPI)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			reqLogger.Info("Workspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Devworkspace spec is invalid: %s", deploymentStatus.Err)
			return reconcile.Result{}, deploymentStatus.Err
		}
		reqLogger.Info("Waiting on deployment to be ready")
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.Conditions[devworkspace.WorkspaceReady] = ""
	timing.SetTime(workspace, timing.DeploymentReady)

	serverReady, err := checkServerStatus(workspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	timing.SetTime(workspace, timing.WorkspaceReady)
	timing.SummarizeStartup(workspace)
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
	ctx context.Context, workspace *devworkspace.DevWorkspace, reqLogger logr.Logger) {
	if timing.IsEnabled() {
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

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Set up indexing https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#setup
	return ctrl.NewControllerManagedBy(mgr).
		For(&devworkspace.DevWorkspace{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Owns(&controllerv1alpha1.Component{}).
		Owns(&controllerv1alpha1.WorkspaceRouting{}).
		WithEventFilter(predicates).
		Complete(r)
}
