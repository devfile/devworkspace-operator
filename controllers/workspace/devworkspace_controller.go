//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"net/http"
	"time"

	devfilevalidation "github.com/devfile/api/v2/pkg/validation"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/workspace/metrics"
	"github.com/devfile/devworkspace-operator/pkg/conditions"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/annotate"
	containerlib "github.com/devfile/devworkspace-operator/pkg/library/container"
	"github.com/devfile/devworkspace-operator/pkg/library/flatten"
	"github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/pkg/provision/metadata"
	"github.com/devfile/devworkspace-operator/pkg/provision/storage"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
	"github.com/go-logr/logr"
	coputil "github.com/redhat-cop/operator-utils/pkg/util"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

	if ok, result, err := r.checkWorkspaceShouldBeStarted(workspace, ctx, reqLogger); !ok {
		return result, err
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{phase: dw.DevWorkspaceStatusStarting}
	reconcileStatus.setConditionTrue(conditions.Started, "DevWorkspace is starting")
	clusterWorkspace := workspace.DeepCopy()

	defer func() (reconcile.Result, error) {
		// Don't accidentally suppress errors by overwriting here; only check for timeout when no error
		// encountered in main reconcile loop.
		if err == nil {
			if timeoutErr := checkForStartTimeout(clusterWorkspace); timeoutErr != nil {
				reconcileResult, err = r.failWorkspace(workspace, timeoutErr.Error(), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
			}
		}
		if reconcileStatus.phase == dw.DevWorkspaceStatusRunning {
			metrics.WorkspaceRunning(clusterWorkspace, reqLogger)
			r.syncStartedAtToCluster(ctx, clusterWorkspace, reqLogger)
		}

		return r.updateWorkspaceStatus(clusterWorkspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	if workspace.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation] == "true" {
		msg, err := r.validateCreatorLabel(clusterWorkspace)
		if err != nil {
			return r.failWorkspace(workspace, msg, metrics.ReasonWorkspaceEngineFailure, reqLogger, &reconcileStatus)
		}
	}

	if _, ok := clusterWorkspace.Annotations[constants.DevWorkspaceStopReasonAnnotation]; ok {
		delete(clusterWorkspace.Annotations, constants.DevWorkspaceStopReasonAnnotation)
		err = r.Update(context.TODO(), clusterWorkspace)
		return reconcile.Result{Requeue: true}, err
	}

	// TODO#185 : Temporarily do devfile flattening in main reconcile loop; this should be moved to a subcontroller.
	flattenHelpers := flatten.ResolverTools{
		WorkspaceNamespace: workspace.Namespace,
		Context:            ctx,
		K8sClient:          r.Client,
		HttpClient:         http.DefaultClient,
	}
	flattenedWorkspace, warnings, err := flatten.ResolveDevWorkspace(&workspace.Spec.Template, flattenHelpers)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}
	if warnings != nil {
		reconcileStatus.setConditionTrue(conditions.DevWorkspaceWarning, flatten.FormatVariablesWarning(warnings))
	} else {
		reconcileStatus.setConditionFalse(conditions.DevWorkspaceWarning, "No warnings in processing DevWorkspace")
	}
	workspace.Spec.Template = *flattenedWorkspace
	reconcileStatus.setConditionTrue(conditions.DevWorkspaceResolved, "Resolved plugins and parents from DevWorkspace")

	// Verify that the devworkspace components are valid after flattening
	components := workspace.Spec.Template.Components
	if components != nil {
		eventErrors := devfilevalidation.ValidateComponents(components)
		if eventErrors != nil {
			return r.failWorkspace(workspace, eventErrors.Error(), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
		}
	}

	storageProvisioner, err := storage.GetProvisioner(workspace)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning storage: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}

	// Set finalizer on DevWorkspace if necessary
	// Note: we need to check the flattened workspace to see if a finalizer is needed, as plugins could require storage
	if storageProvisioner.NeedsStorage(&workspace.Spec.Template) {
		coputil.AddFinalizer(clusterWorkspace, storageCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Add init container to clone projects
	projects.AddProjectClonerComponent(&workspace.Spec.Template)

	devfilePodAdditions, err := containerlib.GetKubeContainersFromDevfile(&workspace.Spec.Template)
	if err != nil {
		return r.failWorkspace(workspace, fmt.Sprintf("Error processing devfile: %s", err), metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
	}

	err = storageProvisioner.ProvisionStorage(devfilePodAdditions, workspace, clusterAPI)
	if err != nil {
		switch storageErr := err.(type) {
		case *storage.NotReadyError:
			reqLogger.Info(storageErr.Message)
			reconcileStatus.setConditionFalse(conditions.StorageReady, fmt.Sprintf("Provisioning storage: %s", storageErr.Message))
			return reconcile.Result{Requeue: true, RequeueAfter: storageErr.RequeueAfter}, nil
		case *storage.ProvisioningError:
			return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning storage: %s", storageErr), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
		default:
			return reconcile.Result{}, storageErr
		}
	}
	reconcileStatus.setConditionTrue(conditions.StorageReady, "Storage ready")

	rbacStatus := wsprovision.SyncRBAC(workspace, clusterAPI)
	if rbacStatus.Err != nil || !rbacStatus.Continue {
		return reconcile.Result{Requeue: true}, rbacStatus.Err
	}

	// Step two: Create routing, and wait for routing to be ready
	routingStatus := wsprovision.SyncRoutingToCluster(workspace, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			return r.failWorkspace(workspace, routingStatus.Message, metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
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
	err = metadata.ProvisionWorkspaceMetadata(devfilePodAdditions, clusterWorkspace, workspace, clusterAPI)
	if err != nil {
		switch provisionErr := err.(type) {
		case *metadata.NotReadyError:
			reqLogger.Info(provisionErr.Message)
			return reconcile.Result{Requeue: true, RequeueAfter: provisionErr.RequeueAfter}, nil
		case *metadata.ProvisioningError:
			return r.failWorkspace(workspace, fmt.Sprintf("Error provisioning metadata configmap: %s", provisionErr), metrics.ReasonInfrastructureFailure, reqLogger, &reconcileStatus)
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
	serviceAcctStatus := wsprovision.SyncServiceAccount(workspace, saAnnotations, clusterAPI)
	if !serviceAcctStatus.Continue {
		if serviceAcctStatus.FailStartup {
			return r.failWorkspace(workspace, serviceAcctStatus.Message, metrics.ReasonBadRequest, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting for workspace ServiceAccount")
		reconcileStatus.setConditionFalse(dw.DevWorkspaceServiceAccountReady, "Waiting for DevWorkspace ServiceAccount")
		if !serviceAcctStatus.Requeue && serviceAcctStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
		return reconcile.Result{Requeue: serviceAcctStatus.Requeue}, serviceAcctStatus.Err
	}
	if wsprovision.NeedsServiceAccountFinalizer(&workspace.Spec.Template) {
		coputil.AddFinalizer(clusterWorkspace, serviceAccountCleanupFinalizer)
		if err := r.Update(ctx, clusterWorkspace); err != nil {
			return reconcile.Result{}, err
		}
	}

	serviceAcctName := serviceAcctStatus.ServiceAccountName
	reconcileStatus.setConditionTrue(dw.DevWorkspaceServiceAccountReady, "DevWorkspace serviceaccount ready")

	pullSecretStatus := wsprovision.PullSecrets(clusterAPI, serviceAcctName, workspace.GetNamespace())
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
	deploymentStatus := wsprovision.SyncDeploymentToCluster(workspace, allPodAdditions, serviceAcctName, clusterAPI)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			failureReason := metrics.DetermineProvisioningFailureReason(deploymentStatus)
			return r.failWorkspace(workspace, deploymentStatus.Info(), failureReason, reqLogger, &reconcileStatus)
		}
		reqLogger.Info("Waiting on deployment to be ready")
		reconcileStatus.setConditionFalse(conditions.DeploymentReady, "Waiting for workspace deployment")
		if !deploymentStatus.Requeue && deploymentStatus.Err == nil {
			return reconcile.Result{RequeueAfter: startingWorkspaceRequeueInterval}, nil
		}
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.setConditionTrue(conditions.DeploymentReady, "DevWorkspace deployment ready")

	serverReady, err := checkServerStatus(clusterWorkspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		reconcileStatus.setConditionFalse(dw.DevWorkspaceReady, "Waiting for editor to start")
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	reconcileStatus.setConditionTrue(dw.DevWorkspaceReady, "")
	reconcileStatus.phase = dw.DevWorkspaceStatusRunning
	return reconcile.Result{}, nil
}
