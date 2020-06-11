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

package workspace

import (
	"context"
	"errors"
	"fmt"
	origLog "log"
	"os"
	"strings"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	"github.com/devfile/devworkspace-operator/internal/cluster"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/pkg/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/devfile/devworkspace-operator/pkg/controller/workspace/provision"
	"github.com/devfile/devworkspace-operator/pkg/controller/workspace/restapis"
	devworkspace "github.com/devfile/kubernetes-api/pkg/apis/workspaces/v1alpha1"
	"github.com/google/uuid"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_workspace")

type currentStatus struct {
	// List of condition types that are true for the current workspace
	Conditions []devworkspace.WorkspaceConditionType
	// Current workspace phase
	Phase devworkspace.WorkspacePhase
}

// Add creates a new Workspace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileWorkspace {
	return &ReconcileWorkspace{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileWorkspace) error {
	// Create a new controller
	c, err := controller.New("workspace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	operatorNamespace, err := k8sutil.GetOperatorNamespace()
	if err == nil {
		config.ConfigMapReference.Namespace = operatorNamespace
	} else if err == k8sutil.ErrRunLocal {
		config.ConfigMapReference.Namespace = os.Getenv("WATCH_NAMESPACE")
		log.Info(fmt.Sprintf("Running operator in local mode; watching namespace %s", config.ConfigMapReference.Namespace))
	} else if err != k8sutil.ErrNoNamespace {
		return err
	}

	err = config.WatchControllerConfig(c, mgr)
	if err != nil {
		return err
	}

	// Check if we're running on OpenShift
	isOS, err := cluster.IsOpenShift()
	if err != nil {
		return err
	}
	config.ControllerCfg.SetIsOpenShift(isOS)

	err = config.ControllerCfg.Validate()
	if err != nil {
		log.Error(err, "Controller configuration is invalid")
		return err
	}

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &devworkspace.DevWorkspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner Workspace
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &devworkspace.DevWorkspace{},
	})
	if err != nil {
		return err
	}

	// Watch for changes in secondary resource Components and requeue the owner workspace
	err = c.Watch(&source.Kind{Type: &controllerv1alpha1.Component{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &devworkspace.DevWorkspace{},
	})

	err = c.Watch(&source.Kind{Type: &controllerv1alpha1.WorkspaceRouting{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &devworkspace.DevWorkspace{},
	})

	// Redirect standard logging to the reconcile's log
	// Necessary as e.g. the plugin broker logs to stdout
	origLog.SetOutput(r)

	return nil
}

// blank assignment to verify that ReconcileWorkspace implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWorkspace{}

// ReconcileWorkspace reconciles a Workspace object
type ReconcileWorkspace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Enable redirecting standard log output to the controller's log
func (r *ReconcileWorkspace) Write(p []byte) (n int, err error) {
	log.Info(string(p))
	return len(p), nil
}

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
func (r *ReconcileWorkspace) Reconcile(request reconcile.Request) (reconcileResult reconcile.Result, err error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Workspace")
	clusterAPI := provision.ClusterAPI{
		Client: r.client,
		Scheme: r.scheme,
		Logger: reqLogger,
	}

	// Fetch the Workspace instance
	workspace := &devworkspace.DevWorkspace{}
	err = r.client.Get(context.TODO(), request.NamespacedName, workspace)
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

	// Ensure workspaceID is set.
	if workspace.Status.WorkspaceId == "" {
		workspaceId, err := getWorkspaceId(workspace)
		if err != nil {
			return reconcile.Result{}, err
		}
		workspace.Status.WorkspaceId = workspaceId
		err = r.client.Status().Update(context.TODO(), workspace)
		return reconcile.Result{Requeue: true}, err
	}

	if !workspace.Spec.Started {
		return r.stopWorkspace(workspace, reqLogger)
	}

	if workspace.Status.Phase == devworkspace.WorkspaceStatusFailed {
		// TODO: Figure out when workspace spec is changed and clear failed status to allow reconcile to continue
		reqLogger.Info("Workspace startup is failed; not attempting to update.")
		return reconcile.Result{}, nil
	}

	// Prepare handling workspace status and condition
	reconcileStatus := currentStatus{
		Phase: devworkspace.WorkspaceStatusStarting,
	}
	defer func() (reconcile.Result, error) {
		return r.updateWorkspaceStatus(workspace, reqLogger, &reconcileStatus, reconcileResult, err)
	}()

	immutable := workspace.Annotations[config.WorkspaceImmutableAnnotation]
	if immutable == "true" && config.ControllerCfg.GetWebhooksEnabled() != "true" {
		reqLogger.Info("Workspace is configured as immutable but webhooks are not enabled.")
		reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
		return reconcile.Result{}, nil
	}

	// Step one: Create components, and wait for their states to be ready.
	componentsStatus := provision.SyncComponentsToCluster(workspace, clusterAPI)
	if !componentsStatus.Continue {
		reqLogger.Info("Waiting on components to be ready")
		return reconcile.Result{Requeue: componentsStatus.Requeue}, componentsStatus.Err
	}
	componentDescriptions := componentsStatus.ComponentDescriptions
	reconcileStatus.Conditions = append(reconcileStatus.Conditions, devworkspace.WorkspaceReady)

	// Only add che rest apis if Theia editor is present in the devfile
	if restapis.IsCheRestApisRequired(workspace.Spec.Template.Components) {
		if !restapis.IsCheRestApisConfigured() {
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			return reconcile.Result{Requeue: false}, errors.New("Che REST API sidecar is not configured but required for used Theia plugin")
		}
		// TODO: first half of provisioning rest-apis
		cheRestApisComponent := restapis.GetCheRestApisComponent(workspace.Name, workspace.Status.WorkspaceId, workspace.Namespace)
		componentDescriptions = append(componentDescriptions, cheRestApisComponent)
	}

	pvcStatus := provision.SyncPVC(workspace, componentDescriptions, r.client, reqLogger)
	if pvcStatus.Err != nil || !pvcStatus.Continue {
		return reconcile.Result{Requeue: true}, err
	}

	rbacStatus := provision.SyncRBAC(workspace, r.client, reqLogger)
	if rbacStatus.Err != nil || !rbacStatus.Continue {
		return reconcile.Result{Requeue: true}, err
	}

	// Step two: Create routing, and wait for routing to be ready
	routingStatus := provision.SyncRoutingToCluster(workspace, componentDescriptions, clusterAPI)
	if !routingStatus.Continue {
		if routingStatus.FailStartup {
			reqLogger.Info("Workspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			return reconcile.Result{}, routingStatus.Err
		}
		reqLogger.Info("Waiting on routing to be ready")
		return reconcile.Result{Requeue: routingStatus.Requeue}, routingStatus.Err
	}
	reconcileStatus.Conditions = append(reconcileStatus.Conditions, devworkspace.WorkspaceRoutingReady)

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
			if configMapStatus.FailStartup {
				reqLogger.Info("Workspace start failed")
				reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
				return reconcile.Result{}, configMapStatus.Err
			}
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
		if serviceAcctStatus.FailStartup {
			reqLogger.Info("Workspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			return reconcile.Result{}, serviceAcctStatus.Err
		}
		reqLogger.Info("Waiting for workspace ServiceAccount")
		return reconcile.Result{Requeue: serviceAcctStatus.Requeue}, serviceAcctStatus.Err
	}
	serviceAcctName := serviceAcctStatus.ServiceAccountName
	reconcileStatus.Conditions = append(reconcileStatus.Conditions, devworkspace.WorkspaceServiceAccountReady)

	// Step five: Create deployment and wait for it to be ready
	deploymentStatus := provision.SyncDeploymentToCluster(workspace, podAdditions, componentDescriptions, serviceAcctName, clusterAPI)
	if !deploymentStatus.Continue {
		if deploymentStatus.FailStartup {
			reqLogger.Info("Workspace start failed")
			reconcileStatus.Phase = devworkspace.WorkspaceStatusFailed
			return reconcile.Result{}, deploymentStatus.Err
		}
		reqLogger.Info("Waiting on deployment to be ready")
		return reconcile.Result{Requeue: deploymentStatus.Requeue}, deploymentStatus.Err
	}
	reconcileStatus.Conditions = append(reconcileStatus.Conditions, devworkspace.WorkspaceReady)

	serverReady, err := checkServerStatus(workspace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !serverReady {
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}
	reconcileStatus.Phase = devworkspace.WorkspaceStatusRunning
	return reconcile.Result{}, nil
}

func (r *ReconcileWorkspace) stopWorkspace(workspace *devworkspace.DevWorkspace, logger logr.Logger) (reconcile.Result, error) {
	workspaceDeployment := &appsv1.Deployment{}
	namespaceName := types.NamespacedName{
		Name:      common.DeploymentName(workspace.Status.WorkspaceId),
		Namespace: workspace.Namespace,
	}
	status := &currentStatus{}
	err := r.client.Get(context.TODO(), namespaceName, workspaceDeployment)
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
		err = r.client.Patch(context.TODO(), workspaceDeployment, patch)
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

func getWorkspaceId(instance *devworkspace.DevWorkspace) (string, error) {
	uid, err := uuid.Parse(string(instance.UID))
	if err != nil {
		return "", err
	}
	return "workspace" + strings.Join(strings.Split(uid.String(), "-")[0:3], ""), nil
}
