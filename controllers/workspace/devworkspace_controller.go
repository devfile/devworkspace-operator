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
	"strconv"
	"strings"
	"time"

	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/metrics"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	wkspConfig "github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/library/env"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	"github.com/devfile/devworkspace-operator/pkg/library/home"
	kubesync "github.com/devfile/devworkspace-operator/pkg/library/kubernetes"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/pkg/library/status"
	"github.com/devfile/devworkspace-operator/pkg/provision/automount"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
	"github.com/devfile/devworkspace-operator/pkg/provision/workspace/rbac"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete
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
	rawWorkspace := &dw.DevWorkspace{}
	err = r.Get(ctx, req.NamespacedName, rawWorkspace)
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

	reconcileStatus := currentStatus{}
	config, err := wkspConfig.ResolveConfigForWorkspace(rawWorkspace, clusterAPI.Client)
	if err != nil {
		reconcileStatus.addWarning(fmt.Sprint("Error applying external DevWorkspace-Operator configuration: ", err.Error()))
		config = wkspConfig.GetGlobalConfig()
	}
	configString := wkspConfig.GetCurrentConfigString(config)
	workspace := &common.DevWorkspaceWithConfig{}
	workspace.DevWorkspace = rawWorkspace
	workspace.Config = config

	reqLogger = reqLogger.WithValues(constants.DevWorkspaceIDLoggerKey, workspace.Status.DevWorkspaceId)
	reqLogger.Info("Reconciling Workspace", "resolvedConfig", configString)

	// Check if the DevWorkspaceRouting instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if workspace.GetDeletionTimestamp() != nil {
		reqLogger.Info("Finalizing DevWorkspace")
		return r.finalize(ctx, reqLogger, workspace)
	}

	// Ensure workspaceID is set.
	if workspace.Status.DevWorkspaceId == "" {
		workspaceId, err := r.getWorkspaceId(ctx, workspace)
		if err != nil {
			workspace.Status.Phase = dw.DevWorkspaceStatusFailed
			workspace.Status.Message = fmt.Sprintf("Failed to set DevWorkspace ID: %s", err.Error())
			return reconcile.Result{}, r.Status().Update(ctx, workspace.DevWorkspace)
		}
		workspace.Status.DevWorkspaceId = workspaceId
		err = r.Status().Update(ctx, workspace.DevWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	// Stop failed workspaces
	if workspace.Status.Phase == devworkspacePhaseFailing && workspace.Spec.Started {
		// If debug annotation is present, leave the deployment in place to let users
		// view logs.
		if workspace.Annotations[constants.DevWorkspaceDebugStartAnnotation] == "true" {
			if isTimeout, err := checkForFailingTimeout(workspace); err != nil {
				return reconcile.Result{}, err
			} else if !isTimeout {
				return reconcile.Result{}, nil
			}
		}

		patch := []byte(`{"spec":{"started": false}}`)
		err := r.Client.Patch(context.Background(), workspace.DevWorkspace, client.RawPatch(types.MergePatchType, patch))
		if err != nil {
			return reconcile.Result{}, err
		}

		// Requeue reconcile to stop workspace
		return reconcile.Result{Requeue: true}, nil
	}

	// Handle stopped workspaces
	if !workspace.Spec.Started {
		r.removeStartedAtFromCluster(ctx, workspace, reqLogger)
		return r.stopWorkspace(ctx, workspace, reqLogger)
	}

	// If this is the first reconcile for a starting workspace, mark it as starting now. This is done outside the regular
	// updateWorkspaceStatus function to ensure it gets set immediately
	if workspace.Status.Phase != dw.DevWorkspaceStatusStarting && workspace.Status.Phase != dw.DevWorkspaceStatusRunning {
		// Set 'Started' condition as early as possible to get accurate timing metrics
		workspace.Status.Phase = dw.DevWorkspaceStatusStarting
		workspace.Status.Message = "Initializing DevWorkspace"
		workspace.Status.Conditions = []dw.DevWorkspaceCondition{
			{
				Type:               conditions.Started,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{Time: clock.Now()},
				Message:            "DevWorkspace is starting",
			},
		}
		err = r.Status().Update(ctx, workspace.DevWorkspace)
		if err == nil {
			metrics.WorkspaceStarted(workspace, reqLogger)
		}
		return reconcile.Result{}, err
	}

	// Prepare handling workspace status and condition
	reconcileStatus.phase = dw.DevWorkspaceStatusStarting
	reconcileStatus.setConditionTrue(conditions.Started, "DevWorkspace is starting")
	clusterWorkspace := &common.DevWorkspaceWithConfig{}
	clusterWorkspace.DevWorkspace = workspace.DevWorkspace.DeepCopy()
	clusterWorkspace.Config = workspace.Config

	defer func() (reconcile.Result, error) {
		// Don't accidentally suppress errors by overwriting here; only check for timeout when no error
		// encountered in main reconcile loop.
		if err == nil {
			if timeoutErr := checkForStartTimeout(clusterWorkspace); timeoutErr != nil {
				// Check if an ignoredUnrecoverableEvent occured and report it alongside the timeout notice
				errMsg := status.CheckForIgnoredWorkspacePodEvents(workspace, clusterAPI)
				if errMsg != "" {
					failureMsg := fmt.Sprintf("%s. Ignored events: %s", timeoutErr.Error(), errMsg)
					reconcileResult = r.failWorkspace(workspace, failureMsg, metrics.DetermineProvisioningFailureReason(errMsg), reqLogger, &reconcileStatus)
				} else {
					reconcileResult = r.failWorkspace(workspace, timeoutErr.Error(), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
				}
			}
		}
		if reconcileStatus.phase == dw.DevWorkspaceStatusRunning {
			// defer to set the startedAt annotation after the status and metrics are updated,
			// since WorkspaceStarted and WorkspaceRunning metrics are not updated if this annotation exists
			defer r.syncStartedAtToCluster(ctx, clusterWorkspace, reqLogger)
		}

		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	if workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation] == "true" {
		msg, err := r.validateCreatorLabel(clusterWorkspace)
		if err != nil {
			return r.failWorkspace(workspace, msg, metrics.ReasonWorkspaceEngineFailure, reqLogger, &reconcileStatus), nil
		}
	}

	if _, ok := clusterWorkspace.Annotations[constants.DevWorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, constants.DevWorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), clusterWorkspace.DevWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	flattenHelpers := flatten.ResolverTools{
		WorkspaceNamespace:          workspace.Namespace,
		Context:                     ctx,
		K8sClient:                   r.Client,
		HttpClient:                  httpClient,
		DefaultResourceRequirements: workspace.Config.Workspace.DefaultContainerResources,
	}

	if wsDefaults.NeedsDefaultTemplate(workspace) {
		wsDefaults.ApplyDefaultTemplate(workspace)
	}

	flattenedWorkspace, warnings, err := flatten.ResolveDevWorkspace(&workspace.Spec.Template, workspace.Spec.Contributions, flattenHelpers)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}
	if warnings != nil {
		reconcileStatus.addWarning(flatten.FormatVariablesWarning(warnings))
	}
	workspace.Spec.Template = *flattenedWorkspace
	reconcileStatus.setConditionTrue(conditions.DevWorkspaceResolved, "Resolved plugins and parents from DevWorkspace")

	// Verify that the devworkspace components are valid after flattening
	components := workspace.Spec.Template.Components
	if components != nil {
		eventErrors := devfilevalidation.ValidateComponents(components)
		if eventErrors != nil {
			return r.failWorkspace(workspace, eventErrors.Error(), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
		}
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning storage: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}

	if storageProvisioner.NeedsStorage(&workspace.Spec.Template) && home.NeedsPersistentHomeDirectory(workspace) {
		workspaceWithHomeVolume, err := home.AddPersistentHomeVolume(workspace)
		if err != nil {
			reconcileStatus.addWarning(fmt.Sprintf("Info: default persistentHome volume is not being used: %s", err.Error()))
		} else {
			workspace.Spec.Template = *workspaceWithHomeVolume
		}
	}

	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if storageProvisioner.NeedsStorage(&workspace.Spec.Template) && !controllerutil.ContainsFinalizer(clusterWorkspace, constants.StorageCleanupFinalizer) {
		controllerutil.AddFinalizer(clusterWorkspace, constants.StorageCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace.DevWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(
		&workspace.Spec.Template,
		workspace.Config.Workspace.ContainerSecurityContext,
		workspace.Config.Workspace.ImagePullPolicy,
		workspace.Config.Workspace.DefaultContainerResources)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}

	// Add common environment variables and env vars defined via workspaceEnv attribute
	if err := env.AddCommonEnvironmentVariables(devfilePodAdditions, clusterWorkspace, &workspace.Spec.Template); err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Failed to process workspace environment variables: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}

	// Validate that projects, dependentProjects, and starterProjects do not collide
	if err := projects.ValidateAllProjects(&workspace.Spec.Template); err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Invalid devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}
	// Add init container to clone projects
	projectCloneOptions := projects.Options{
		Image:     workspace.Config.Workspace.ProjectCloneConfig.Image,
		Env:       env.GetEnvironmentVariablesForProjectClone(workspace),
		Resources: workspace.Config.Workspace.ProjectCloneConfig.Resources,
	}
	if workspace.Config.Workspace.ProjectCloneConfig.ImagePullPolicy != "" {
		projectCloneOptions.PullPolicy = config.Workspace.ProjectCloneConfig.ImagePullPolicy
	} else {
		projectCloneOptions.PullPolicy = corev1.PullPolicy(config.Workspace.ImagePullPolicy)
	}
	if projectClone, err := projects.GetProjectCloneInitContainer(&workspace.Spec.Template, projectCloneOptions, workspace.Config.Routing.ProxyConfig); err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Failed to set up project-clone init container: %s", err), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus), nil
	} else if projectClone != nil {
		devfilePodAdditions.InitContainers = append(devfilePodAdditions.InitContainers, *projectClone)
	}

	// Add ServiceAccount tokens into devfile containers
	if err := wsprovision.ProvisionServiceAccountTokensInto(devfilePodAdditions, workspace); err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Failed to mount ServiceAccount tokens to workspace: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
	}

	// Add automount resources into devfile containers
	err = automount.ProvisionAutoMountResourcesInto(devfilePodAdditions, clusterAPI, workspace.Namespace)
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Failed to process automount resources", metrics.ReasonBadRequest, reqLogger, &reconcileStatus); shouldReturn {
		return reconcileResult, reconcileErr
	}

	err = storageProvisioner.ProvisionStorage(devfilePodAdditions, workspace, clusterAPI)
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error provisioning storage", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
		reconcileStatus.setConditionFalse(conditions.StorageReady, fmt.Sprintf("Provisioning storage: %s", err.Error()))
		return reconcileResult, reconcileErr
	}
	reconcileStatus.setConditionTrue(conditions.StorageReady, "Storage ready")

	// Add finalizer to ensure workspace rolebinding gets cleaned up when workspace
	// is deleted.
	if !controllerutil.ContainsFinalizer(clusterWorkspace, constants.RBACCleanupFinalizer) {
		controllerutil.AddFinalizer(clusterWorkspace, constants.RBACCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace.DevWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}
	err = rbac.SyncRBAC(workspace, clusterAPI)
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error provisioning rbac", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
		return reconcileResult, reconcileErr
	}

	// Step two: Create routing, and wait for routing to be ready
	routingPodAdditions, exposedEndpoints, statusMsg, err := wsprovision.SyncRoutingToCluster(workspace, clusterAPI)
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Failed to set up networking for workspace", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
		reqLogger.Info("Waiting on routing to be ready")
		if statusMsg == "" {
			statusMsg = "Preparing networking"
		}
		reconcileStatus.setConditionFalse(dw.DevWorkspaceRoutingReady, statusMsg)
		return reconcileResult, reconcileErr
	}
	reconcileStatus.setConditionTrue(dw.DevWorkspaceRoutingReady, "Networking ready")

	statusOk, err := syncWorkspaceMainURL(clusterWorkspace, exposedEndpoints, clusterAPI)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !statusOk {
		reqLogger.Info("Updating workspace status")
		return reconcile.Result{Requeue: true}, nil
	}

	annotate.AddURLAttributesToEndpoints(&workspace.Spec.Template, exposedEndpoints)

	// Step three: provision a configmap on the cluster to mount the flattened devfile in deployment containers
	err = metadata.ProvisionWorkspaceMetadata(devfilePodAdditions, clusterWorkspace, workspace, clusterAPI)
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error provisioning metadata configmap", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
		return reconcileResult, reconcileErr
	}

	// Step four: Collect all workspace deployment contributions
	allPodAdditions := []controllerv1alpha1.PodAdditions{*devfilePodAdditions}
	if routingPodAdditions != nil {
		allPodAdditions = append(allPodAdditions, *routingPodAdditions)
	}

	// Step five: Prepare workspace ServiceAccount
	var serviceAcctName string
	if *workspace.Config.Workspace.ServiceAccount.DisableCreation {
		if workspace.Config.Workspace.ServiceAccount.ServiceAccountName == "" {
			return r.failWorkspace(workspace, "Configured ServiceAccount name is required when ServiceAccount creation is disabled", metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
		}
		if routingPodAdditions != nil && routingPodAdditions.Annotations != nil {
			// This routingClass defines annotations to be applied to the workspace SA, which we cannot do since
			// we are not managing the SA. This feature is not used in DWO anymore and was previously used to support
			// the openshift-oauth routingClass.
			return r.failWorkspace(workspace, fmt.Sprintf("Disabling ServiceAccount creation is incompatible with workspace routingClass %s", workspace.Spec.RoutingClass), metrics.ReasonBadRequest, reqLogger, &reconcileStatus), nil
		}
		// We have to assume the ServiceAccount exists as even if it does exist we generally can't access it -- DWO only caches
		// ServiceAccounts with the devworkspace ID label.
		serviceAcctName = workspace.Config.Workspace.ServiceAccount.ServiceAccountName
		reconcileStatus.setConditionTrue(dw.DevWorkspaceServiceAccountReady, fmt.Sprintf("Using existing ServiceAccount %s", serviceAcctName))
	} else {
		saAnnotations := map[string]string{}
		if routingPodAdditions != nil {
			saAnnotations = routingPodAdditions.ServiceAccountAnnotations
		}
		saName, err := wsprovision.SyncServiceAccount(workspace, saAnnotations, clusterAPI)
		if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error setting up DevWorkspace ServiceAccount", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
			reqLogger.Info("Waiting for workspace ServiceAccount")
			reconcileStatus.setConditionFalse(dw.DevWorkspaceServiceAccountReady, "Waiting for DevWorkspace ServiceAccount")
			return reconcileResult, reconcileErr
		}
		serviceAcctName = saName
		reconcileStatus.setConditionTrue(dw.DevWorkspaceServiceAccountReady, "DevWorkspace serviceaccount ready")
	}

	pullSecretPodAdditions, err := wsprovision.PullSecrets(clusterAPI, serviceAcctName, workspace.GetNamespace())
	if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error getting DevWorkspace image pull secrets", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
		reconcileStatus.setConditionFalse(conditions.PullSecretsReady, "Waiting for DevWorkspace pull secrets")
		return reconcileResult, reconcileErr
	}

	if pullSecretPodAdditions != nil && pullSecretPodAdditions.PullSecrets != nil {
		allPodAdditions = append(allPodAdditions, *pullSecretPodAdditions)
		reconcileStatus.setConditionTrue(conditions.PullSecretsReady, "DevWorkspace secrets ready")
	}

	if kubesync.HasKubelikeComponent(workspace) {
		err := kubesync.HandleKubernetesComponents(workspace, clusterAPI)
		if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error provisioning workspace Kubernetes components", metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus); shouldReturn {
			reconcileStatus.setConditionFalse(conditions.KubeComponentsReady, "Waiting for DevWorkspace Kubernetes components to be created on cluster")
			return reconcileResult, reconcileErr
		}
		reconcileStatus.setConditionTrue(conditions.KubeComponentsReady, "Kubernetes components ready")
	}

	// Step six: Create deployment and wait for it to be ready
	if err := wsprovision.SyncDeploymentToCluster(workspace, allPodAdditions, serviceAcctName, clusterAPI); err != nil {
		if shouldReturn, reconcileResult, reconcileErr := r.checkDWError(workspace, err, "Error creating DevWorkspace deployment", metrics.DetermineProvisioningFailureReason(err.Error()), reqLogger, &reconcileStatus); shouldReturn {
			reqLogger.Info("Waiting on deployment to be ready")
			reconcileStatus.setConditionFalse(conditions.DeploymentReady, "Waiting for workspace deployment")
			return reconcileResult, reconcileErr
		}
	}
	reconcileStatus.setConditionTrue(conditions.DeploymentReady, "DevWorkspace deployment ready")

	serverReady, serverStatusCode, err := checkServerStatus(clusterWorkspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		reqLogger.Info("Main URL server not ready", "status-code", serverStatusCode)
		reconcileStatus.setConditionFalse(dw.DevWorkspaceReady, "Waiting for editor to start")
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	reqLogger.Info("Workspace is running")
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
	return r.updateWorkspaceStatus(workspace, logger, &status, reconcile.Result{}, nil)
}

func (r *DevWorkspaceReconciler) doStop(ctx context.Context, workspace *common.DevWorkspaceWithConfig, logger logr.Logger) (stopped bool, err error) {
	if pointer.BoolDeref(workspace.Config.Workspace.CleanupOnStop, false) {
		logger.Info("Cleaning up workspace-owned objects")
		requeue, err := r.deleteWorkspaceOwnedObjects(ctx, workspace)
		return !requeue, err
	}

	// Update DevWorkspaceRouting to have `devworkspace-started` annotation "false"
	routing := &controllerv1alpha1.DevWorkspaceRouting{}
	routingNN := types.NamespacedName{
		Name:      common.DevWorkspaceRoutingName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err = r.Get(ctx, routingNN, routing)
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

	workspaceDeployment := &appsv1.Deployment{}
	deployNN := types.NamespacedName{
		Name:      common.DeploymentName(workspace.Status.DevWorkspaceId),
		Namespace: workspace.Namespace,
	}
	err = r.Get(ctx, deployNN, workspaceDeployment)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	replicas := workspaceDeployment.Spec.Replicas
	if replicas == nil || *replicas > 0 {
		logger.Info("Stopping workspace")
		err = wsprovision.ScaleDeploymentToZero(ctx, workspace, r.Client)
		if err != nil && !k8sErrors.IsConflict(err) {
			return false, err
		}
		return false, nil
	}
	return workspaceDeployment.Status.Replicas == 0, nil
}

// failWorkspace marks a workspace as failed by setting relevant fields in the status struct.
// These changes are not synced to cluster immediately, and are intended to be synced to the cluster via a deferred function
// in the main reconcile loop. If needed, changes can be flushed to the cluster immediately via `updateWorkspaceStatus()`
func (r *DevWorkspaceReconciler) failWorkspace(workspace *common.DevWorkspaceWithConfig, msg string, reason metrics.FailureReason, logger logr.Logger, status *currentStatus) reconcile.Result {
	logger.Info("DevWorkspace failed to start: " + msg)
	status.phase = devworkspacePhaseFailing
	status.setConditionTrueWithReason(dw.DevWorkspaceFailedStart, msg, string(reason))
	if workspace.Spec.Started {
		return reconcile.Result{Requeue: true}
	}
	return reconcile.Result{}
}

func (r *DevWorkspaceReconciler) checkDWError(workspace *common.DevWorkspaceWithConfig, err error, failHint string, reason metrics.FailureReason, logger logr.Logger, status *currentStatus) (shouldReturn bool, res reconcile.Result, returnErr error) {
	if err == nil {
		return false, reconcile.Result{}, nil
	}
	switch detailErr := err.(type) {
	case *dwerrors.RetryError:
		logger.Info(detailErr.Error())
		return true, reconcile.Result{Requeue: true, RequeueAfter: detailErr.RequeueAfter}, nil
	case *dwerrors.FailError:
		return true, r.failWorkspace(workspace, fmt.Sprintf("%s: %s", failHint, detailErr), metrics.ReasonInfrastructureFailure, logger, status), nil
	case *dwerrors.WarningError:
		status.addWarning(detailErr.Error())
		return false, reconcile.Result{}, nil
	default:
		return true, reconcile.Result{}, err
	}
}

func (r *DevWorkspaceReconciler) syncStartedAtToCluster(
	ctx context.Context, workspace *common.DevWorkspaceWithConfig, reqLogger logr.Logger) {

	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}

	if _, hasStartedAtAnnotation := workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation]; hasStartedAtAnnotation {
		return
	}

	workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation] = strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	if err := r.Update(ctx, workspace.DevWorkspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply started-at annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply started-at annotation to devworkspace")
		}
	}
}

func (r *DevWorkspaceReconciler) removeStartedAtFromCluster(
	ctx context.Context, workspace *common.DevWorkspaceWithConfig, reqLogger logr.Logger) {
	if workspace.Annotations == nil {
		// No annotations, nothing to do
		return
	}
	if _, ok := workspace.Annotations[constants.DevWorkspaceStartedAtAnnotation]; !ok {
		// Annotation has already been deleted
		return
	}

	delete(workspace.Annotations, constants.DevWorkspaceStartedAtAnnotation)
	if err := r.Update(ctx, workspace.DevWorkspace); err != nil {
		if k8sErrors.IsConflict(err) {
			reqLogger.Info("Got conflict when trying to apply timing annotations to workspace")
		} else {
			reqLogger.Error(err, "Error trying to apply timing annotations to devworkspace")
		}
	}
}

func (r *DevWorkspaceReconciler) getWorkspaceId(ctx context.Context, workspace *common.DevWorkspaceWithConfig) (string, error) {
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

func (r *DevWorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupHttpClients()

	maxConcurrentReconciles, err := wkspConfig.GetMaxConcurrentReconciles()
	if err != nil {
		return err
	}

	var emptyMapper = func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{}
	}

	configWatcher := builder.WithPredicates(wkspConfig.Predicates())
	automountWatcher := builder.WithPredicates(automountPredicates)

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
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.runningWorkspacesHandler), automountWatcher).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(r.runningWorkspacesHandler), automountWatcher).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, handler.EnqueueRequestsFromMapFunc(r.runningWorkspacesHandler), automountWatcher).
		Watches(&source.Kind{Type: &controllerv1alpha1.DevWorkspaceOperatorConfig{}}, handler.EnqueueRequestsFromMapFunc(emptyMapper), configWatcher).
		WithEventFilter(devworkspacePredicates).
		WithEventFilter(podPredicates).
		Complete(r)
}
