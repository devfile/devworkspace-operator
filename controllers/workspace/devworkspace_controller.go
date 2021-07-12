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
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	registry "github.com/devfile/devworkspace-operator/pkg/library/flatten/internal_registry"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"github.com/devfile/devworkspace-operator/pkg/timing"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;delete

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
	workspace := &dw.DevWorkspace{}
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
	reqLogger = reqLogger.WithValues(constants.DevWorkspaceIDLoggerKey, workspace.Status.DevWorkspaceId)
	reqLogger.Info("Reconciling Workspace")

	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if workspace.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspace")
		return r.finalize(ctx, reqLogger, workspace)
	}

	// Ensure workspaceID is set.
	if workspace.Status.DevWorkspaceId == "" {
		workspaceId, err := getWorkspaceId(workspace)
		if err != nil {
			return reconcile.Result{}, err
		}
		workspace.Status.DevWorkspaceId = workspaceId
		workspace.Status.Phase = dw.DevWorkspaceStatusStarting
		workspace.Status.Message = "Initializing DevWorkspace"
		err = r.Status().Update(ctx, workspace)
		return reconcile.Result{Requeue: true}, err
	}

	// Stop failed workspaces
	if workspace.Status.Phase == devworkspacePhaseFailing && workspace.Spec.Started {
		// If debug annotation is present, leave the deployment in place to let users
		// view logs.
		if workspace.Annotations[constants.DevWorkspaceDebugStartAnnotation] == "true" {
			return reconcile.Result{}, nil
		}

		patch := []byte(`{"spec":{"started": false}}`)
		err := r.Client.Patch(context.Background(), workspace, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			return reconcile.Result{}, err
		}

		// Requeue reconcile to stop workspace
		return reconcile.Result{Requeue: true}, nil
	}

	// Handle stopped workspaces
	if !workspace.Spec.Started {
		timing.ClearAnnotations(workspace)
		r.syncTimingToCluster(ctx, workspace, map[string]string{}, reqLogger)
		return r.stopWorkspace(workspace, reqLogger)
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{phase: dw.DevWorkspaceStatusStarting}
	clusterWorkspace := workspace.DeepCopy()
	timingInfo := map[string]string{}
	timing.SetTime(timingInfo, timing.DevWorkspaceStarted)
	defer func() (reconcile.Result, error) {
		r.syncTimingToCluster(ctx, clusterWorkspace, timingInfo, reqLogger)
		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	if workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation] == "true" {
		msg, err := r.validateCreatorLabel(clusterWorkspace)
		if err != nil {
			return r.failWorkspace(workspace, msg, reqLogger, &reconcileStatus)
		}
	}

	if _, ok := clusterWorkspace.Annotations[constants.DevWorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, constants.DevWorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), clusterWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	timing.SetTime(timingInfo, timing.ComponentsCreated)
	// TODO#185 : Temporarily do devfile flattening in main reconcile loop; this should be moved to a subcontroller.
	flattenHelpers := flatten.ResolverTools{
		WorkspaceNamespace: workspace.Namespace,
		Context:            ctx,
		K8sClient:          r.Client,
		InternalRegistry:   &registry.InternalRegistryImpl{},
		HttpClient:         http.DefaultClient,
	}
	flattenedWorkspace, err := flatten.ResolveDevWorkspace(&workspace.Spec.Template, flattenHelpers)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), reqLogger, &reconcileStatus)
	}
	workspace.Spec.Template = *flattenedWorkspace
	reconcileStatus.setConditionTrue(DevWorkspaceResolved, "Resolved plugins and parents from DevWorkspace")

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning storage: %s", err), reqLogger, &reconcileStatus)
	}

	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if isFinalizerNecessary(workspace, storageProvisioner) {
		coputil.AddFinalizer(clusterWorkspace, storageCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Add init container to clone projects
	projects.AddProjectClonerComponent(&workspace.Spec.Template)

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(&workspace.Spec.Template)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), reqLogger, &reconcileStatus)
	}

	err = storageProvisioner.ProvisionStorage(devfilePodAdditions, workspace, clusterAPI)
	if err != nil {
		switch storageErr := err.(type) {
		case *storage.NotReadyError:
			reqLogger.Info(storageErr.Message)
			reconcileStatus.setConditionFalse(StorageReady, fmt.Sprintf("Provisioning storage: %s", storageErr.Message))
			return reconcile.Result{Requeue: true, RequeueAfter: storageErr.RequeueAfter}, nil
		case *storage.ProvisioningError:
			return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning storage: %s", storageErr), reqLogger, &reconcileStatus)
		default:
			return reconcile.Result{}, storageErr
		}
	}
	reconcileStatus.setConditionTrue(StorageReady, "Storage ready")

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
			return r.failWorkspace(workspace, routingStatus.Message, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on routing to be ready")
		message := "Preparing networking"
		if routingStatus.Message != "" {
			message = routingStatus.Message
		}
		reconcileStatus.setConditionFalse(dw.DevWorkspaceRoutingReady, message)
		return reconcile.Result{Requeue: routingStatus.Requeue}, routingStatus.Err
	}
	reconcileStatus.setConditionTrue(dw.DevWorkspaceRoutingReady, "Networking ready")
	timing.SetTime(timingInfo, timing.RoutingReady)

	statusOk, err := syncWorkspaceMainURL(clusterWorkspace, routingStatus.ExposedEndpoints, clusterAPI)
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
			return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning metadata configmap: %s", provisionErr), reqLogger, &reconcileStatus)
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
		reconcileStatus.setConditionFalse(dw.DevWorkspaceServiceAccountReady, "Waiting for DevWorkspace ServiceAccount")
		return reconcile.Result{Requeue: serviceAcctStatus.Requeue}, serviceAcctStatus.Err
	}
	serviceAcctName := serviceAcctStatus.ServiceAccountName
	reconcileStatus.setConditionTrue(dw.DevWorkspaceServiceAccountReady, "DevWorkspace serviceaccount ready")

	pullSecretStatus := provision.PullSecrets(clusterAPI)
	if !pullSecretStatus.Continue {
		reconcileStatus.setConditionFalse(PullSecretsReady, "Waiting for DevWorkspace pull secrets")
		return reconcile.Result{Requeue: pullSecretStatus.Requeue}, pullSecretStatus.Err
	}
	allPodAdditions = append(allPodAdditions, pullSecretStatus.PodAdditions)
	reconcileStatus.setConditionTrue(PullSecretsReady, "DevWorkspace secrets ready")

	// Step six: Create deployment and wait for it to be ready
	timing.SetTime(timingInfo, timing.DeploymentCreated)
	deploymentStatus := provision.SyncDeploymentToCluster(workspace, allPodAdditions, serviceAcctName, clusterAPI)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			return r.failWorkspace(workspace, deploymentStatus.Info(), reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on deployment to be ready")
		reconcileStatus.setConditionFalse(DeploymentReady, "Waiting for workspace deployment")
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.setConditionTrue(DeploymentReady, "DevWorkspace deployment ready")
	timing.SetTime(timingInfo, timing.DeploymentReady)

	serverReady, err := checkServerStatus(clusterWorkspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		reconcileStatus.setConditionFalse(dw.DevWorkspaceReady, "Waiting for editor to start")
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	timing.SetTime(timingInfo, timing.DevWorkspaceReady)
	timing.SummarizeStartup(clusterWorkspace)
	reconcileStatus.setConditionTrue(dw.DevWorkspaceReady, "")
	reconcileStatus.phase = dw.DevWorkspaceStatusRunning
	return reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) stopWorkspace(workspace *dw.DevWorkspace, logger logr.Logger) (reconcile.Result, error) {
	status := currentStatus{phase: dw.DevWorkspaceStatusStopping}
	if workspace.Status.Phase == devworkspacePhaseFailing || workspace.Status.Phase == dw.DevWorkspaceStatusFailed {
		status.phase = workspace.Status.Phase
		failedCondition := getConditionByType(workspace.Status.Conditions, dw.DevWorkspaceFailedStart)
		if failedCondition != nil {
			status.setCondition(dw.DevWorkspaceFailedStart, *failedCondition)
		}
	}

	stopped, err := r.doStop(workspace, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if stopped {
		switch status.phase {
		case devworkspacePhaseFailing, dw.DevWorkspaceStatusFailed:
			status.phase = dw.DevWorkspaceStatusFailed
		default:
			status.phase = dw.DevWorkspaceStatusStopped
		}
	}
	return r.updateWorkspaceStatus(workspace, logger, &status, reconcile.Result{}, nil)
}

func (r *DevWorkspaceReconciler) doStop(workspace *dw.DevWorkspace, logger logr.Logger) (stopped bool, err error) {
	workspaceDeployment := &appsv1.Deployment{}
	namespaceName := types.NamespacedName{
		Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err = r.Get(context.TODO(), namespaceName, workspaceDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	replicas := workspaceDeployment.Spec.Replicas
	if replicas == nil || *replicas > 0 {
		logger.Info("Stopping workspace")
		err = provision.ScaleDeploymentToZero(workspace, r.Client)
		if err != nil && !k8sErrors.IsConflict(err) {
			return false, err
		}
		return false, nil
	}

	if workspaceDeployment.Status.Replicas == 0 {
		return true, nil
	}
	return false, nil
}

// failWorkspace marks a workspace as failed by setting relevant fields in the status struct.
// These changes are not synced to cluster immediately, and are intended to be synced to the cluster via a deferred function
// in the main reconcile loop. If needed, changes can be flushed to the cluster immediately via `updateWorkspaceStatus()`
func (r *DevWorkspaceReconciler) failWorkspace(workspace *dw.DevWorkspace, msg string, logger logr.Logger, status *currentStatus) (reconcile.Result, error) {
	logger.Info("DevWorkspace failed to start: " + msg)
	status.phase = devworkspacePhaseFailing
	status.setConditionTrue(dw.DevWorkspaceFailedStart, msg)
	if workspace.Spec.Started {
		return reconcile.Result{Requeue: true}, nil
	}
	return reconcile.Result{}, nil
}

func (r *DevWorkspaceReconciler) syncTimingToCluster(
	ctx context.Context, workspace *dw.DevWorkspace, timingInfo map[string]string, reqLogger logr.Logger) {
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

func getWorkspaceId(instance *dw.DevWorkspace) (string, error) {
	uid, err := uuid.Parse(string(instance.UID))
	if err != nil {
		return "", err
	}
	return "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""), nil
}

// Mapping the pod to the devworkspace
func dwRelatedPodsHandler() handler.EventHandler {
	podToDW := func(mapObj handler.MapObject) []reconcile.Request {
		meta := mapObj.Meta
		labels := meta.GetLabels()
		if _, ok := labels[constants.DevWorkspaceNameLabel]; !ok {
			return nil
		}

		//If the dewworkspace label does not exist, do no reconcile
		if _, ok := labels[constants.DevWorkspaceIDLabel]; !ok {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      labels[constants.DevWorkspaceNameLabel],
					Namespace: meta.GetNamespace(),
				},
			},
		}
	}
	return &handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(podToDW)}
}

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	maxConcurrentReconciles, err := config.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	// TODO: Set up indexing https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#setup
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&dw.DevWorkspace{}).
		// List DevWorkspaceTemplates as owned to enable updating workspaces when templates
		// are changed; this should be moved to whichever controller is responsible for flattening
		// DevWorkspaces
		Owns(&dw.DevWorkspaceTemplate{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Owns(&controllerv1alpha1.DevWorkspaceRouting{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, dwRelatedPodsHandler()).
		WithEventFilter(predicates).
		WithEventFilter(podPredicates).
		Complete(r)
}
