//
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
//

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/metrics"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/library/env"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/pkg/provision/automount"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
	"github.com/devfile/devworkspace-operator/pkg/timing"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	wsDefaults "github.com/devfile/devworkspace-operator/pkg/library/defaults"
)

const (
	startingWorkspaceRequeueInterval = 5 * time.Second
)

// DevWorkspaceReconciler reconciles a DevWorkspace object
type DevWorkspaceReconciler struct {
	client.Client
	NonCachingClient client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
}

/////// CRD-related RBAC roles
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=*,verbs=*
// +kubebuilder:rbac:groups=controller.devfile.io,resources=*,verbs=*
/////// Required permissions for controller
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments;replicasets,verbs=*
// +kubebuilder:rbac:groups="",resources=pods;serviceaccounts;secrets;configmaps;persistentvolumeclaims,verbs=*
// +kubebuilder:rbac:groups="",resources=namespaces;events,verbs=get;list;watch
// +kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;create;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews;localsubjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=oauth.openshift.io,resources=oauthclients,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies,verbs=get,resourceNames=cluster
// +kubebuilder:rbac:groups=apps,resourceNames=devworkspace-controller,resources=deployments/finalizers,verbs=update
/////// Required permissions for workspace ServiceAccount
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups=apps;extensions,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps;extensions,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,resourceNames=workspace-credentials-secret,verbs=get;create;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,resourceNames=workspace-preferences-configmap,verbs=get;create;patch;delete
// +kubebuilder:rbac:groups="metrics.k8s.io",resources=pods,verbs=get;list;watch

func (r *DevWorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcileResult ctrl.Result, err error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	clusterAPI := sync.ClusterAPI{
		Client:           r.Client,
		NonCachingClient: r.NonCachingClient,
		Scheme:           r.Scheme,
		Logger:           reqLogger,
		Ctx:              ctx,
	}

	// Fetch the Workspace instance
	workspaceWithConfig := &common.DevWorkspaceWithConfig{}
	workspaceWithConfig.Config = *config.InternalConfig
	err = r.Get(ctx, req.NamespacedName, workspaceWithConfig)
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
	reqLogger = reqLogger.WithValues(constants.DevWorkspaceIDLoggerKey, workspaceWithConfig.Status.DevWorkspaceId)
	reqLogger.Info("Reconciling Workspace")

	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if workspaceWithConfig.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspace")
		return r.finalize(ctx, reqLogger, workspaceWithConfig)
	}

	// Ensure workspaceID is set.
	if workspaceWithConfig.Status.DevWorkspaceId == "" {
		workspaceId, err := r.getWorkspaceId(ctx, &workspaceWithConfig.DevWorkspace)
		if err != nil {
			workspaceWithConfig.Status.Phase = dw.DevWorkspaceStatusFailed
			workspaceWithConfig.Status.Message = fmt.Sprintf("Failed to set DevWorkspace ID: %s", err.Error())
			return reconcile.Result{}, r.Status().Update(ctx, workspaceWithConfig)
		}
		workspaceWithConfig.Status.DevWorkspaceId = workspaceId
		err = r.Status().Update(ctx, workspaceWithConfig)
		return reconcile.Result{Requeue: true}, err
	}

	// Stop failed workspaces
	if workspaceWithConfig.Status.Phase == devworkspacePhaseFailing && workspaceWithConfig.Spec.Started {
		// If debug annotation is present, leave the deployment in place to let users
		// view logs.
		if workspaceWithConfig.Annotations[constants.DevWorkspaceDebugStartAnnotation] == "true" {
			if isTimeout, err := checkForFailingTimeout(workspaceWithConfig); err != nil {
				return reconcile.Result{}, err
			} else if !isTimeout {
				return reconcile.Result{}, nil
			}
		}

		patch := []byte(`{"spec":{"started": false}}`)
		err := r.Client.Patch(context.Background(), workspaceWithConfig, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			return reconcile.Result{}, err
		}

		// Requeue reconcile to stop workspace
		return reconcile.Result{Requeue: true}, nil
	}

	// Handle stopped workspaces
	if !workspaceWithConfig.Spec.Started {
		timing.ClearAnnotations(&workspaceWithConfig.DevWorkspace)
		r.removeStartedAtFromCluster(ctx, &workspaceWithConfig.DevWorkspace, reqLogger)
		r.syncTimingToCluster(ctx, &workspaceWithConfig.DevWorkspace, map[string]string{}, reqLogger)
		return r.stopWorkspace(ctx, workspaceWithConfig, reqLogger)
	}

	// If this is the first reconcile for a starting workspace, mark it as starting now. This is done outside the regular
	// updateWorkspaceStatus function to ensure it gets set immediately
	if workspaceWithConfig.Status.Phase != dw.DevWorkspaceStatusStarting && workspaceWithConfig.Status.Phase != dw.DevWorkspaceStatusRunning {
		// Set 'Started' condition as early as possible to get accurate timing metrics
		workspaceWithConfig.Status.Phase = dw.DevWorkspaceStatusStarting
		workspaceWithConfig.Status.Message = "Initializing DevWorkspace"
		workspaceWithConfig.Status.Conditions = []dw.DevWorkspaceCondition{
			{
				Type:               conditions.Started,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: clock.Now()},
				Message:            "DevWorkspace is starting",
			},
		}
		err = r.Status().Update(ctx, workspaceWithConfig)
		if err == nil {
			metrics.WorkspaceStarted(&workspaceWithConfig.DevWorkspace, reqLogger)
		}
		return reconcile.Result{}, err
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{phase: dw.DevWorkspaceStatusStarting}
	reconcileStatus.setConditionTrue(conditions.Started, "DevWorkspace is starting")
	clusterWorkspace := workspaceWithConfig.DevWorkspace.DeepCopy()
	timingInfo := map[string]string{}
	timing.SetTime(timingInfo, timing.DevWorkspaceStarted)

	defer func() (reconcile.Result, error) {
		r.syncTimingToCluster(ctx, clusterWorkspace, timingInfo, reqLogger)

		// Don't accidentally suppress errors by overwriting here; only check for timeout when no error
		// encountered in main reconcile loop.
		if err == nil {
			if timeoutErr := checkForStartTimeout(clusterWorkspace, workspaceWithConfig.Config); timeoutErr != nil {
				reconcileResult, err = r.failWorkspace(&workspaceWithConfig.DevWorkspace, timeoutErr.Error(), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
			}
		}
		if reconcileStatus.phase == dw.DevWorkspaceStatusRunning {
			metrics.WorkspaceRunning(&workspaceWithConfig.DevWorkspace, reqLogger)
			r.syncStartedAtToCluster(ctx, clusterWorkspace, reqLogger)
		}

		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	if workspaceWithConfig.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation] == "true" {
		msg, err := r.validateCreatorLabel(clusterWorkspace)
		if err != nil {
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, msg, metrics.ReasonWorkspaceEngineFailure, reqLogger, &reconcileStatus)
		}
	}

	if _, ok := clusterWorkspace.Annotations[constants.DevWorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, constants.DevWorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), clusterWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	timing.SetTime(timingInfo, timing.ComponentsCreated)
	flattenHelpers := flatten.ResolverTools{
		WorkspaceNamespace: workspaceWithConfig.Namespace,
		Context:            ctx,
		K8sClient:          r.Client,
		HttpClient:         httpClient,
	}

	if wsDefaults.NeedsDefaultTemplate(workspaceWithConfig) {
		wsDefaults.ApplyDefaultTemplate(workspaceWithConfig)
	}

	flattenedWorkspace, warnings, err := flatten.ResolveDevWorkspace(&workspaceWithConfig.Spec.Template, flattenHelpers)
	if err != nil {
		return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}
	if warnings != nil {
		reconcileStatus.setConditionTrue(conditions.DevWorkspaceWarning, flatten.FormatVariablesWarning(warnings))
	} else {
		reconcileStatus.setConditionFalse(conditions.DevWorkspaceWarning, "No warnings in processing DevWorkspace")
	}
	workspaceWithConfig.Spec.Template = *flattenedWorkspace
	reconcileStatus.setConditionTrue(conditions.DevWorkspaceResolved, "Resolved plugins and parents from DevWorkspace")

	// Verify that the devworkspace components are valid after flattening
	components := workspaceWithConfig.Spec.Template.Components
	if components != nil {
		eventErrors := devfilevalidation.ValidateComponents(components)
		if eventErrors != nil {
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, eventErrors.Error(), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
		}
	}

	storageProvisioner, err := storage.GetProvisioner(&workspaceWithConfig.DevWorkspace)
	if err != nil {
		return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Error provisioning storage: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}

	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if storageProvisioner.NeedsStorage(&workspaceWithConfig.Spec.Template) {
		coputil.AddFinalizer(clusterWorkspace, constants.StorageCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(&workspaceWithConfig.Spec.Template, workspaceWithConfig.Config)
	if err != nil {
		return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}

	// Add common environment variables and env vars defined via workspaceEnv attribute
	if err := env.AddCommonEnvironmentVariables(devfilePodAdditions, clusterWorkspace, &workspaceWithConfig.Spec.Template, workspaceWithConfig.Config); err != nil {
		return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Failed to process workspace environment variables: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}

	// Add init container to clone projects
	if projectClone, err := projects.GetProjectCloneInitContainer(workspaceWithConfig); err != nil {
		return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Failed to set up project-clone init container: %s", err), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
	} else if projectClone != nil {
		devfilePodAdditions.InitContainers = append(devfilePodAdditions.InitContainers, *projectClone)
	}

	// Add automount resources into devfile containers
	if err := automount.ProvisionAutoMountResourcesInto(devfilePodAdditions, clusterAPI, workspaceWithConfig.Namespace); err != nil {
		var autoMountErr *automount.AutoMountError
		if errors.As(err, &autoMountErr) {
			if autoMountErr.IsFatal {
				return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Failed to process automount resources: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
			}
			reqLogger.Info(autoMountErr.Error())
			return reconcile.Result{Requeue: true}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	err = storageProvisioner.ProvisionStorage(devfilePodAdditions, workspaceWithConfig, clusterAPI)
	if err != nil {
		switch storageErr := err.(type) {
		case *storage.NotReadyError:
			reqLogger.Info(storageErr.Message)
			reconcileStatus.setConditionFalse(conditions.StorageReady, fmt.Sprintf("Provisioning storage: %s", storageErr.Message))
			return reconcile.Result{Requeue: true, RequeueAfter: storageErr.RequeueAfter}, nil
		case *storage.ProvisioningError:
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Error provisioning storage: %s", storageErr), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
		default:
			return reconcile.Result{}, storageErr
		}
	}
	reconcileStatus.setConditionTrue(conditions.StorageReady, "Storage ready")

	timing.SetTime(timingInfo, timing.ComponentsReady)

	rbacStatus := wsprovision.SyncRBAC(&workspaceWithConfig.DevWorkspace, clusterAPI)
	if rbacStatus.Err != nil || !rbacStatus.Continue {
		return reconcile.Result{Requeue: true}, rbacStatus.Err
	}

	// Step two: Create routing, and wait for routing to be ready
	timing.SetTime(timingInfo, timing.RoutingCreated)
	routingStatus := wsprovision.SyncRoutingToCluster(&workspaceWithConfig.DevWorkspace, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, routingStatus.Message, metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on routing to be ready")
		message := "Preparing networking"
		if routingStatus.Message != "" {
			message = routingStatus.Message
		}
		reconcileStatus.setConditionFalse(dw.DevWorkspaceRoutingReady, message)
		if !routingStatus.Requeue && routingStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
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

	annotate.AddURLAttributesToEndpoints(&workspaceWithConfig.Spec.Template, routingStatus.ExposedEndpoints)

	// Step three: provision a configmap on the cluster to mount the flattened devfile in deployment containers
	err = metadata.ProvisionWorkspaceMetadata(devfilePodAdditions, clusterWorkspace, &workspaceWithConfig.DevWorkspace, clusterAPI)
	if err != nil {
		switch provisionErr := err.(type) {
		case *metadata.NotReadyError:
			reqLogger.Info(provisionErr.Message)
			return reconcile.Result{Requeue: true, RequeueAfter: provisionErr.RequeueAfter}, nil
		case *metadata.ProvisioningError:
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, fmt.Sprintf("Error provisioning metadata configmap: %s", provisionErr), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
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
	serviceAcctStatus := wsprovision.SyncServiceAccount(&workspaceWithConfig.DevWorkspace, saAnnotations, clusterAPI)
	if !serviceAcctStatus.Continue {
		if serviceAcctStatus.FailStartup {
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, serviceAcctStatus.Message, metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting for workspace ServiceAccount")
		reconcileStatus.setConditionFalse(dw.DevWorkspaceServiceAccountReady, "Waiting for DevWorkspace ServiceAccount")
		if !serviceAcctStatus.Requeue && serviceAcctStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
		return reconcile.Result{Requeue: serviceAcctStatus.Requeue}, serviceAcctStatus.Err
	}
	if wsprovision.NeedsServiceAccountFinalizer(&workspaceWithConfig.Spec.Template) {
		coputil.AddFinalizer(clusterWorkspace, constants.ServiceAccountCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	serviceAcctName := serviceAcctStatus.ServiceAccountName
	reconcileStatus.setConditionTrue(dw.DevWorkspaceServiceAccountReady, "DevWorkspace serviceaccount ready")

	pullSecretStatus := wsprovision.PullSecrets(clusterAPI, serviceAcctName, workspaceWithConfig.GetNamespace())
	if !pullSecretStatus.Continue {
		reconcileStatus.setConditionFalse(conditions.PullSecretsReady, "Waiting for DevWorkspace pull secrets")
		if !pullSecretStatus.Requeue && pullSecretStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
		return reconcile.Result{Requeue: pullSecretStatus.Requeue}, pullSecretStatus.Err
	}
	allPodAdditions = append(allPodAdditions, pullSecretStatus.PodAdditions)
	reconcileStatus.setConditionTrue(conditions.PullSecretsReady, "DevWorkspace secrets ready")

	// Step six: Create deployment and wait for it to be ready
	timing.SetTime(timingInfo, timing.DeploymentCreated)
	deploymentStatus := wsprovision.SyncDeploymentToCluster(&workspaceWithConfig.DevWorkspace, allPodAdditions, serviceAcctName, clusterAPI, &workspaceWithConfig.Config)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			failureReason := metrics.DetermineProvisioningFailureReason(deploymentStatus)
			return r.failWorkspace(&workspaceWithConfig.DevWorkspace, deploymentStatus.Info(), failureReason, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on deployment to be ready")
		reconcileStatus.setConditionFalse(conditions.DeploymentReady, "Waiting for workspace deployment")
		if !deploymentStatus.Requeue && deploymentStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.setConditionTrue(conditions.DeploymentReady, "DevWorkspace deployment ready")
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

func (r *DevWorkspaceReconciler) stopWorkspace(ctx context.Context, workspace *common.DevWorkspaceWithConfig, logger logr.Logger) (reconcile.Result, error) {
	status := currentStatus{phase: dw.DevWorkspaceStatusStopping}
	if workspace.Status.Phase == devworkspacePhaseFailing || workspace.Status.Phase == dw.DevWorkspaceStatusFailed {
		status.phase = workspace.Status.Phase
		failedCondition := conditions.GetConditionByType(workspace.Status.Conditions, dw.DevWorkspaceFailedStart)
		if failedCondition != nil {
			status.setCondition(dw.DevWorkspaceFailedStart, *failedCondition)
		}
	}

	stopped, err := r.doStop(ctx, workspace, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if stopped {
		switch status.phase {
		case devworkspacePhaseFailing, dw.DevWorkspaceStatusFailed:
			status.phase = dw.DevWorkspaceStatusFailed
			status.setConditionFalse(conditions.Started, "Workspace stopped due to error")
		default:
			status.phase = dw.DevWorkspaceStatusStopped
			status.setConditionFalse(conditions.Started, "Workspace is stopped")
		}
	}
	if stoppedBy, ok := workspace.Annotations[constants.DevWorkspaceStopReasonAnnotation]; ok {
		logger.Info("Workspace stopped with reason", "stopped-by", stoppedBy)
	}
	return r.updateWorkspaceStatus(&workspace.DevWorkspace, logger, &status, reconcile.Result{}, nil)
}

func (r *DevWorkspaceReconciler) doStop(ctx context.Context, workspaceWithConfig *common.DevWorkspaceWithConfig, logger logr.Logger) (stopped bool, err error) {
	workspaceDeployment := &appsv1.Deployment{}
	namespaceName := types.NamespacedName{
		Name:      common.DeploymentName(workspaceWithConfig.Status.DevWorkspaceId),
		Namespace: workspaceWithConfig.Namespace,
	}
	err = r.Get(ctx, namespaceName, workspaceDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// Update DevWorkspaceRouting to have `devworkspace-started` annotation "false"
	routing := &controllerv1alpha1.DevWorkspaceRouting{}
	routingRef := types.NamespacedName{
		Name:      common.DevWorkspaceRoutingName(workspaceWithConfig.Status.DevWorkspaceId),
		Namespace: workspaceWithConfig.Namespace,
	}
	err = r.Get(ctx, routingRef, routing)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return false, err
		}
	} else if routing.Annotations != nil && routing.Annotations[constants.DevWorkspaceStartedStatusAnnotation] != "false" {
		routing.Annotations[constants.DevWorkspaceStartedStatusAnnotation] = "false"
		err := r.Update(ctx, routing)
		if err != nil {
			if k8sErrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
	}

	// CleanupOnStop should never be nil as a default is always set
	if workspaceWithConfig.Config.Workspace.CleanupOnStop == nil || !*workspaceWithConfig.Config.Workspace.CleanupOnStop {
		replicas := workspaceDeployment.Spec.Replicas
		if replicas == nil || *replicas > 0 {
			logger.Info("Stopping workspace")
			err = wsprovision.ScaleDeploymentToZero(ctx, &workspaceWithConfig.DevWorkspace, r.Client)
			if err != nil && !k8sErrors.IsConflict(err) {
				return false, err
			}
			return false, nil
		}

		return workspaceDeployment.Status.Replicas == 0, nil
	} else {
		logger.Info("Cleaning up workspace-owned objects")
		requeue, err := r.deleteWorkspaceOwnedObjects(ctx, &workspaceWithConfig.DevWorkspace)
		return !requeue, err
	}
}

// failWorkspace marks a workspace as failed by setting relevant fields in the status struct.
// These changes are not synced to cluster immediately, and are intended to be synced to the cluster via a deferred function
// in the main reconcile loop. If needed, changes can be flushed to the cluster immediately via `updateWorkspaceStatus()`
func (r *DevWorkspaceReconciler) failWorkspace(workspace *dw.DevWorkspace, msg string, reason metrics.FailureReason, logger logr.Logger, status *currentStatus) (reconcile.Result, error) {
	logger.Info("DevWorkspace failed to start: " + msg)
	status.phase = devworkspacePhaseFailing
	status.setConditionTrueWithReason(dw.DevWorkspaceFailedStart, msg, string(reason))
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

func (r *DevWorkspaceReconciler) syncStartedAtToCluster(
	ctx context.Context, workspace *dw.DevWorkspace, reqLogger logr.Logger) {

	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}

	if _, hasStartedAtAnnotation := workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation]; hasStartedAtAnnotation {
		return
	}

	workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation] = timing.CurrentTime()
	if err := r.Update(ctx, workspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply started-at annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply started-at annotation to devworkspace")
		}
	}
}

func (r *DevWorkspaceReconciler) removeStartedAtFromCluster(
	ctx context.Context, workspace *dw.DevWorkspace, reqLogger logr.Logger) {
	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}
	delete(workspace.Annotations, constants.DevWorkspaceStartedAtAnnotation)
	if err := r.Update(ctx, workspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply timing annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply timing annotations to devworkspace")
		}
	}
}

func (r *DevWorkspaceReconciler) getWorkspaceId(ctx context.Context, workspace *dw.DevWorkspace) (string, error) {
	if idOverride := workspace.Annotations[constants.WorkspaceIdOverrideAnnotation]; idOverride != "" {
		if len(idOverride) > 25 {
			return "", fmt.Errorf("maximum length for DevWorkspace ID override is 25 characters")
		}
		dwList := &dw.DevWorkspaceList{}
		if err := r.Client.List(ctx, dwList, &client.ListOptions{Namespace: workspace.Namespace}); err != nil {
			return "", fmt.Errorf("failed to check DevWorkspace ID override: %w", err)
		}
		for _, existing := range dwList.Items {
			if existing.Status.DevWorkspaceId == idOverride {
				return "", fmt.Errorf("DevWorkspace ID specified in override already exists in namespace")
			}
		}
		return idOverride, nil
	} else {
		uid, err := uuid.Parse(string(workspace.UID))
		if err != nil {
			return "", err
		}
		return "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""), nil
	}
}

// Mapping the pod to the devworkspace
func dwRelatedPodsHandler(obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if _, ok := labels[constants.DevWorkspaceNameLabel]; !ok {
		return []reconcile.Request{}
	}

	//If the dewworkspace label does not exist, do no reconcile
	if _, ok := labels[constants.DevWorkspaceIDLabel]; !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      labels[constants.DevWorkspaceNameLabel],
				Namespace: obj.GetNamespace(),
			},
		},
	}
}

func (r *DevWorkspaceReconciler) dwPVCHandler(obj client.Object) []reconcile.Request {
	// Check if PVC is owned by a DevWorkspace (per-workspace storage case)
	for _, ownerref := range obj.GetOwnerReferences() {
		if ownerref.Kind != "DevWorkspace" {
			continue
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      ownerref.Name,
					Namespace: obj.GetNamespace(),
				},
			},
		}
	}

	// TODO: Address this usage of global config?
	// Otherwise, check if common PVC is deleted to make sure all DevWorkspaces see it happen
	if obj.GetName() != config.Workspace.PVCName || obj.GetDeletionTimestamp() == nil {
		// We're looking for a deleted common PVC
		return []reconcile.Request{}
	}
	dwList := &dw.DevWorkspaceList{}
	if err := r.Client.List(context.Background(), dwList, &client.ListOptions{Namespace: obj.GetNamespace()}); err != nil {
		return []reconcile.Request{}
	}
	var reconciles []reconcile.Request
	for _, workspace := range dwList.Items {
		storageType := workspace.Spec.Template.Attributes.GetString(constants.DevWorkspaceStorageTypeAttribute, nil)
		if storageType == constants.CommonStorageClassType || storageType == constants.PerUserStorageClassType || storageType == "" {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workspace.GetName(),
					Namespace: workspace.GetNamespace(),
				},
			})
		}
	}
	return reconciles
}

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupHttpClients()

	maxConcurrentReconciles, err := config.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	var emptyMapper = func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{}
	}

	var configWatcher builder.WatchesOption = builder.WithPredicates(config.Predicates())

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
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(dwRelatedPodsHandler)).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(r.dwPVCHandler)).
		Watches(&source.Kind{Type: &controllerv1alpha1.DevWorkspaceOperatorConfig{}}, handler.EnqueueRequestsFromMapFunc(emptyMapper), configWatcher).
		WithEventFilter(predicates).
		WithEventFilter(podPredicates).
		Complete(r)
}
