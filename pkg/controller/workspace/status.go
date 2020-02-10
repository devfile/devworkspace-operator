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
	"encoding/json"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
	"github.com/go-logr/logr"
	"time"

	"context"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"reflect"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/source"

	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/log"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

var (
	ownedObjectEventPrefix = "workspace-owned/"
)

func getOwningWorkspace(clt client.Client, obj metav1.Object, mgr manager.Manager) metav1.Object {
	if ownerRef := metav1.GetControllerOf(obj); ownerRef != nil {
		switch ownerRef.Kind {
		case "Workspace":
			workspace := &workspacev1alpha1.Workspace{}
			err := clt.Get(context.TODO(), client.ObjectKey{
				Name:      ownerRef.Name,
				Namespace: obj.GetNamespace(),
			}, workspace)
			if err != nil {
				if !errors.IsNotFound(err) {
					Log.Error(err, "")
				}
				return nil
			}
			return workspace
		case "ReplicaSet":
			labels := obj.GetLabels()
			if deploymentName := labels["deployment"]; deploymentName != "" {
				deployment := &appsv1.Deployment{}
				err := clt.Get(context.TODO(), client.ObjectKey{
					Name:      deploymentName,
					Namespace: obj.GetNamespace(),
				}, deployment)
				if err != nil {
					if !errors.IsNotFound(err) {
						Log.Error(err, "")
					}
					return nil
				}
				return getOwningWorkspace(clt, deployment, mgr)
			}
		}
	}
	return nil
}

func watchStatus(ctr controller.Controller, mgr manager.Manager) error {
	for _, obj := range []runtime.Object{
		&appsv1.Deployment{},
		&corev1.Pod{},
		&workspacev1alpha1.WorkspaceRouting{},
	} {
		var mapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
			var requests []reconcile.Request
			if owningWorkspace := getOwningWorkspace(mgr.GetClient(), obj.Meta, mgr); owningWorkspace != nil {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: owningWorkspace.GetNamespace(),
						Name:      ownedObjectEventPrefix + owningWorkspace.GetName(),
					},
				})
			} else if pod, isPod := obj.Object.(*corev1.Pod); isPod {
				workspaceName := pod.GetLabels()[WorkspaceNameLabel]
				if workspaceName != "" {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: pod.GetNamespace(),
							Name:      ownedObjectEventPrefix + workspaceName,
						},
					})
				}
			}
			return requests
		}

		checkOwner := func(obj metav1.Object) bool {
			if obj.GetLabels() != nil && obj.GetLabels()[WorkspaceIDLabel] != "" {
				return true
			}
			return false
		}
		err := ctr.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: mapper,
		}, predicate.Funcs{
			UpdateFunc: func(evt event.UpdateEvent) bool {
				if checkOwner(evt.MetaNew) {
					if _, isPod := evt.ObjectNew.(*corev1.Pod); isPod {
						return true
					}
					if _, isWorkspaceRouting := evt.ObjectNew.(*workspacev1alpha1.WorkspaceRouting); isWorkspaceRouting {
						return true
					}
				}
				return false
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(evt event.DeleteEvent) bool {
				if checkOwner(evt.Meta) {
					if _, isDeployment := evt.Object.(*appsv1.Deployment); isDeployment {
						return true
					}
					if _, isPod := evt.Object.(*corev1.Pod); isPod {
						return true
					}
				}
				return false
			},
			GenericFunc: func(evt event.GenericEvent) bool {
				return false
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileWorkspace) updateStatusAfterWorkspaceChange(rs *reconcileStatus) {
	existingPhase := rs.workspace.Status.Phase
	if rs != nil && rs.workspace != nil {
		if rs.workspace.Status.AdditionalInfo == nil {
			rs.workspace.Status.AdditionalInfo = map[string]string{}
		}
		modifiedStatus := false
		if rs.failure != "" {
			rs.workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseFailed
			for _, conditionType := range []workspacev1alpha1.WorkspaceConditionType{
				workspacev1alpha1.WorkspaceConditionScheduled,
				workspacev1alpha1.WorkspaceConditionInitialized,
				workspacev1alpha1.WorkspaceConditionReady,
			} {
				setWorkspaceCondition(&rs.workspace.Status, *newWorkspaceCondition(
					conditionType,
					corev1.ConditionFalse,
					workspacev1alpha1.WorkspaceConditionReconcileFailureReason,
					rs.failure,
				))
			}
			clearCondition(&rs.workspace.Status, workspacev1alpha1.WorkspaceConditionStopped)
			modifiedStatus = true
		}
		if rs.wkspProps != nil {
			if rs.wkspProps.Started {
				if rs.changedWorkspaceObjects || rs.createdWorkspaceObjects {
					clearCondition(&rs.workspace.Status, workspacev1alpha1.WorkspaceConditionStopped)
					rs.workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseStarting
					modifiedStatus = true
				}
			} else {
				clearConditions(&rs.workspace.Status,
					workspacev1alpha1.WorkspaceConditionScheduled,
					workspacev1alpha1.WorkspaceConditionInitialized,
					workspacev1alpha1.WorkspaceConditionReady,
				)
				if rs.cleanedWorkspaceObjects {
					setWorkspaceCondition(&rs.workspace.Status, *newWorkspaceCondition(
						workspacev1alpha1.WorkspaceConditionStopped,
						corev1.ConditionFalse,
						workspacev1alpha1.WorkspaceConditionStoppingReason,
						"User stopped the workspace",
					))
					rs.workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseStopping
					modifiedStatus = true
				}
			}
			if modifiedStatus {
				rs.workspace.Status.WorkspaceId = rs.wkspProps.WorkspaceId
			}
		}

		if rs.componentInstanceStatuses == nil {
			delete(rs.workspace.Status.AdditionalInfo, ComponentStatusesAdditionalInfo)
		} else {

			statusesAnnotation, err := json.Marshal(rs.componentInstanceStatuses)
			if err != nil {
				Log.Error(err, "")
			}
			rs.workspace.Status.AdditionalInfo[ComponentStatusesAdditionalInfo] = string(statusesAnnotation)
		}

		rs.ReqLogger.V(1).Info("Status Update After Workspace Change : ", "status", rs.workspace.Status)
		err := r.Status().Update(context.TODO(), rs.workspace)
		if err != nil {
			Log.Error(err, "")
		}
		if existingPhase != rs.workspace.Status.Phase {
			rs.ReqLogger.Info("Phase: " + string(existingPhase) + " => " + string(rs.workspace.Status.Phase))
		}
	}
}

func (r *ReconcileWorkspace) updateFromWorkspaceRouting(routing *workspacev1alpha1.WorkspaceRouting, workspace *workspacev1alpha1.Workspace) error {
	if workspace.Status.AdditionalInfo == nil {
		workspace.Status.AdditionalInfo = map[string]string{}
	}
	if routing.Status.Phase != workspacev1alpha1.WorkspaceRoutingReady {
		delete(workspace.Status.AdditionalInfo, RuntimeAdditionalInfo)
		workspace.Status.IdeUrl = ""
	} else {

		statusesAnnotation := workspace.Status.AdditionalInfo[ComponentStatusesAdditionalInfo]
		if statusesAnnotation == "" {
			Log.Error(nil, "statusesAnnotation is empty !")
		}

		var statuses []ComponentInstanceStatus
		err := json.Unmarshal([]byte(statusesAnnotation), &statuses)
		if err != nil {
			Log.Error(err, "")
		}

		var commands []model.CheWorkspaceCommand
		machines := map[string]model.CheWorkspaceMachine{}

		for _, status := range statuses {
			commands = append(commands, status.ContributedRuntimeCommands...)
			for machineName, description := range status.Containers {
				machineExposedEndpoints := routing.Status.ExposedEndpoints[machineName]
				machineServers := map[string]model.CheWorkspaceServer{}
				for _, endpoint := range machineExposedEndpoints {
					machineServer := model.CheWorkspaceServer{
						Status:     model.UnknownServerStatus,
						URL:        &endpoint.Url,
						Attributes: map[workspacev1alpha1.EndpointAttribute]string{},
					}
					for name, val := range endpoint.Attributes {
						serverAttributeName := name
						serverAttributeValue := val
						if name == workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE {
							serverAttributeName = "internal"
							if val == "true" {
								serverAttributeValue = "false"
							} else {
								serverAttributeValue = "true"
							}
						}
						machineServer.Attributes[serverAttributeName] = serverAttributeValue
					}
					machineServers[endpoint.Name] = machineServer
					if endpoint.Attributes[workspacev1alpha1.TYPE_ENDPOINT_ATTRIBUTE] == "ide" {
						workspace.Status.IdeUrl = endpoint.Url
					}
				}
				machines[machineName] = model.CheWorkspaceMachine{
					Servers:    machineServers,
					Attributes: description.Attributes,
				}
			}
		}

		defaultEnv := "default"
		wsRuntime := model.CheWorkspaceRuntime{
			ActiveEnv: &defaultEnv,
			Commands:  commands,
			Machines:  machines,
		}

		runtimeAnnotation, err := json.Marshal(wsRuntime)
		if err != nil {
			return err
		}

		workspace.Status.AdditionalInfo[RuntimeAdditionalInfo] = string(runtimeAnnotation)
	}

	return nil
}

func (r *ReconcileWorkspace) updateStatusFromOwnedObjects(workspace *workspacev1alpha1.Workspace, reqLogger logr.Logger) (reconcile.Result, error) {
	existingPhase := workspace.Status.Phase
	reconcileResult := reconcile.Result{}

	workspace.Status.Members.Ready = []string{}
	workspace.Status.Members.Unready = []string{}
	if workspace.Status.AdditionalInfo == nil {
		workspace.Status.AdditionalInfo = map[string]string{}
	}

	for _, list := range []runtime.Object{
		&corev1.PodList{},
		&workspacev1alpha1.WorkspaceRoutingList{},
	} {
		// TODO Change this to look for objects owned by the workspace CR
		err := r.List(context.TODO(), list,
			client.InNamespace(workspace.GetNamespace()),
			client.MatchingLabels{WorkspaceIDLabel: workspace.Status.WorkspaceId})
		if err != nil {
			Log.Error(err, "Failed to list workspaceRoutings for workspace %s in namespace %s", workspace.GetName(), workspace.GetNamespace())
			return reconcile.Result{Requeue: true, RequeueAfter: 1}, err
		}
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Addr().Interface()
			if itemPod, isPod := item.(*corev1.Pod); isPod {
				podOriginalName, originalNameFound := itemPod.GetLabels()[CheOriginalNameLabel]
				if !originalNameFound {
					podOriginalName = "Unknown"
				}
				if podOriginalName == CheOriginalName {
					for _, container := range itemPod.Status.ContainerStatuses {
						if container.Ready {
							workspace.Status.Members.Ready = append(workspace.Status.Members.Ready, container.Name)
						} else {
							workspace.Status.Members.Unready = append(workspace.Status.Members.Unready, container.Name)
						}
					}
					if workspace.Spec.Started {
						copyPodConditions(&itemPod.Status, &workspace.Status)
						clearCondition(&workspace.Status, workspacev1alpha1.WorkspaceConditionStopped)
						_, workspaceCondition := getWorkspaceCondition(&workspace.Status, workspacev1alpha1.WorkspaceConditionReady)
						if workspaceCondition != nil && workspaceCondition.Status == corev1.ConditionTrue {
							workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseRunning
						}
					} else {
						reconcileResult = reconcile.Result{Requeue: true, RequeueAfter: 1}
					}
				}
			}
			if itemRouting, isWorkspaceRouting := item.(*workspacev1alpha1.WorkspaceRouting); isWorkspaceRouting {
				err := r.updateFromWorkspaceRouting(itemRouting, workspace)
				if err != nil {
					Log.Error(err, "Failed to propagate workspaceRouting into workspace %s in namespace %s", workspace.GetName(), workspace.GetNamespace())
					return reconcile.Result{Requeue: true, RequeueAfter: 1}, err
				}
			}
		}
	}
	if !workspace.Spec.Started {
		podList := &corev1.PodList{}
		err := r.List(context.TODO(), podList,
			client.InNamespace(workspace.GetNamespace()),
			client.MatchingLabels{WorkspaceIDLabel: workspace.Status.WorkspaceId})
		if err == nil && len(podList.Items) == 0 {
			workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseStopped
			setWorkspaceCondition(&workspace.Status, *newWorkspaceCondition(
				workspacev1alpha1.WorkspaceConditionStopped,
				corev1.ConditionTrue,
				"",
				"",
			))
			clearConditions(&workspace.Status,
				workspacev1alpha1.WorkspaceConditionScheduled,
				workspacev1alpha1.WorkspaceConditionInitialized,
				workspacev1alpha1.WorkspaceConditionReady,
			)
		}
	}
	Log.V(1).Info("Status Update After Change To Owned Objects : ", "status", workspace.Status)
	r.Status().Update(context.TODO(), workspace)

	if existingPhase != workspace.Status.Phase {
		reqLogger.Info("Phase: " + string(existingPhase) + " => " + string(workspace.Status.Phase))
	}
	return reconcileResult, nil
}

var podConditionTypeToWorkspaceConditionType = map[corev1.PodConditionType]workspacev1alpha1.WorkspaceConditionType{
	corev1.PodScheduled:   workspacev1alpha1.WorkspaceConditionScheduled,
	corev1.PodInitialized: workspacev1alpha1.WorkspaceConditionInitialized,
	corev1.PodReady:       workspacev1alpha1.WorkspaceConditionReady,
}

func copyPodConditions(podStatus *corev1.PodStatus, workspaceStatus *workspacev1alpha1.WorkspaceStatus) {
	for _, podConditionType := range []corev1.PodConditionType{
		corev1.PodScheduled,
		corev1.PodInitialized,
		corev1.PodReady,
	} {
		_, podCondition := getPodCondition(podStatus, podConditionType)
		if podCondition != nil {
			workspaceConditionType, typeFound := podConditionTypeToWorkspaceConditionType[podCondition.Type]
			if typeFound {
				setWorkspaceCondition(workspaceStatus, *newWorkspaceCondition(
					workspaceConditionType,
					podCondition.Status,
					podCondition.Reason,
					podCondition.Message))
			} else {
				clearCondition(workspaceStatus, workspaceConditionType)
			}
		}
	}
}

func clearConditions(ws *workspacev1alpha1.WorkspaceStatus, types ...workspacev1alpha1.WorkspaceConditionType) {
	for _, t := range types {
		pos, _ := getWorkspaceCondition(ws, t)
		if pos == -1 {
			continue
		}
		ws.Conditions = append(ws.Conditions[:pos], ws.Conditions[pos+1:]...)
	}
}

func clearCondition(ws *workspacev1alpha1.WorkspaceStatus, t workspacev1alpha1.WorkspaceConditionType) {
	pos, _ := getWorkspaceCondition(ws, t)
	if pos == -1 {
		return
	}
	ws.Conditions = append(ws.Conditions[:pos], ws.Conditions[pos+1:]...)
}

func setWorkspaceCondition(ws *workspacev1alpha1.WorkspaceStatus, c workspacev1alpha1.WorkspaceCondition) {
	pos, cp := getWorkspaceCondition(ws, c.Type)
	if cp != nil &&
		cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
		return
	}

	if cp != nil {
		ws.Conditions[pos] = c
	} else {
		ws.Conditions = append(ws.Conditions, c)
	}
}

func getWorkspaceCondition(status *workspacev1alpha1.WorkspaceStatus, t workspacev1alpha1.WorkspaceConditionType) (int, *workspacev1alpha1.WorkspaceCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func newWorkspaceCondition(condType workspacev1alpha1.WorkspaceConditionType, status corev1.ConditionStatus, reason, message string) *workspacev1alpha1.WorkspaceCondition {
	now := time.Now()
	return &workspacev1alpha1.WorkspaceCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Time{now},
		Reason:             reason,
		Message:            message,
	}
}

func getPodCondition(status *corev1.PodStatus, t corev1.PodConditionType) (int, *corev1.PodCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}
