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

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"
	shimlib "github.com/devfile/devworkspace-operator/pkg/library/shim"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"github.com/devfile/devworkspace-operator/pkg/timing"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
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
		Ctx:    ctx,
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
	reqLogger = reqLogger.WithValues(constants.WorkspaceIDLoggerKey, workspace.Status.WorkspaceId)
	reqLogger.Info("Reconciling Workspace")

	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
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
	defer func() (reconcile.Result, error) {
		r.syncTimingToCluster(ctx, clusterWorkspace, timingInfo, reqLogger)
		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	if workspace.Annotations[constants.WorkspaceRestrictedAccessAnnotation] == "true" {
		msg, err := r.validateCreatorLabel(clusterWorkspace)
		if err != nil {
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = msg
			return reconcile.Result{}, err
		}
	}

	if _, ok := clusterWorkspace.Annotations[constants.WorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, constants.WorkspaceStopReasonAnnotation)
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
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		// TODO: Handle error more elegantly
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error processing devfile: %s", err)
		return reconcile.Result{}, nil
	}
	workspace.Spec.Template = *flattenedWorkspace
	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if isFinalizerNecessary(workspace) {
		coputil.AddFinalizer(clusterWorkspace, storageCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(workspace.Spec.Template)
	if err != nil {
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error processing devfile: %s", err)
		return reconcile.Result{}, nil
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		reqLogger.Info("DevWorkspace start failed")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error provisioning storage: %s", err)
		return reconcile.Result{}, nil
	}
	err = storageProvisioner.ProvisionStorage(devfilePodAdditions, workspace, clusterAPI)
	if err != nil {
		switch storageErr := err.(type) {
		case *storage.NotReadyError:
			reqLogger.Info(storageErr.Message)
			return reconcile.Result{Requeue: true, RequeueAfter: storageErr.RequeueAfter}, nil
		case *storage.ProvisioningError:
			reqLogger.Info(fmt.Sprintf("DevWorkspace start failed: %s", storageErr))
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error provisioning storage: %s", storageErr)
			return reconcile.Result{}, nil
		default:
			return reconcile.Result{}, storageErr
		}
	}

	shimlib.FillDefaultEnvVars(devfilePodAdditions, *workspace)

	reconcileStatus.Conditions[devworkspace.WorkspaceComponentsReady] = ""
	timing.SetTime(timingInfo, timing.ComponentsReady)

	rbacStatus := provision.SyncRBAC(workspace, r.Client, reqLogger)
	if rbacStatus.Err != nil || !rbacStatus.Continue {
		return reconcile.Result{Requeue: true}, rbacStatus.Err
	}

	// Step two: Create routing, and wait for routing to be ready
	timing.SetTime(timingInfo, timing.RoutingCreated)
	routingStatus := provision.SyncRoutingToCluster(workspace, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			reqLogger.Info("DevWorkspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			// TODO: Propagate failure reason from devWorkspaceRouting
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = "Failed to install network objects required for devworkspace"
			return reconcile.Result{}, routingStatus.Err
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

	annotate.AddURLAttributesToEndpoints(&workspace.Spec.Template, routingStatus.ExposedEndpoints)

	// Step three: provision a configmap on the cluster to mount the flattened devfile in deployment containers
	err = metadata.ProvisionWorkspaceMetadata(devfilePodAdditions, clusterWorkspace, workspace, &clusterAPI)
	if err != nil {
		switch provisionErr := err.(type) {
		case *metadata.NotReadyError:
			reqLogger.Info(provisionErr.Message)
			return reconcile.Result{Requeue: true, RequeueAfter: provisionErr.RequeueAfter}, nil
		case *metadata.ProvisioningError:
			reqLogger.Info(fmt.Sprintf("DevWorkspace start failed: %s", provisionErr))
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			reconcileStatus.Conditions[devworkspace.WorkspaceFailedStart] = fmt.Sprintf("Error provisioning metadata configmap: %s", provisionErr)
			return reconcile.Result{}, nil
		default:
			return reconcile.Result{}, provisionErr
		}
	}

	// Step four: Collect all workspace deployment contributions
	allPodAdditions := []controllerv1alpha1.PodAdditions{*devfilePodAdditions}
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

	pullSecretStatus := provision.PullSecrets(clusterAPI)
	if !pullSecretStatus.Continue {
		return reconcile.Result{Requeue: pullSecretStatus.Requeue}, pullSecretStatus.Err
	}
	allPodAdditions = append(allPodAdditions, pullSecretStatus.PodAdditions)
	reconcileStatus.Conditions[PullSecretsReadyCondition] = ""

	// Step six: Create deployment and wait for it to be ready
	timing.SetTime(timingInfo, timing.DeploymentCreated)
	deploymentStatus := provision.SyncDeploymentToCluster(workspace, allPodAdditions, serviceAcctName, clusterAPI)
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
		Owns(&controllerv1alpha1.DevWorkspaceRouting{}).
		WithEventFilter(predicates).
		Complete(r)
}
