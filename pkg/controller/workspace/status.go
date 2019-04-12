package workspace

import (
	"time"
	"encoding/json"
	//	"strings"
	"context"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"reflect"

	//	"github.com/google/go-cmp/cmp"
	//	"github.com/google/go-cmp/cmp/cmpopts"
	//	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	//	brokerCfg "github.com/eclipse/che-plugin-broker/cfg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	//	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	//	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
				if ! errors.IsNotFound(err) {
					log.Error(err, "")
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
						if ! errors.IsNotFound(err) {
							log.Error(err, "")
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
		&extensionsv1beta1.Ingress{},
	} {
		var mapper handler.ToRequestsFunc = func(obj handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}
			if owningWorkspace := getOwningWorkspace(mgr.GetClient(), obj.Meta, mgr); owningWorkspace != nil {
				requests = append(requests, reconcile.Request{
					types.NamespacedName{
						Namespace: owningWorkspace.GetNamespace(),
						Name:      ownedObjectEventPrefix + owningWorkspace.GetName(),
					},
				})
			} else if pod, isPod := obj.Object.(*corev1.Pod); isPod {
				workspaceName := pod.GetLabels()["che.workspace_name"]
				if workspaceName != "" {
					requests = append(requests, reconcile.Request{
						types.NamespacedName{
							Namespace: pod.GetNamespace(),
							Name:      ownedObjectEventPrefix + workspaceName,
						},
					})
				}
			}
			return requests
		}

		checkOwner := func(obj metav1.Object) bool {
			if obj.GetLabels() != nil && obj.GetLabels()["che.workspace_id"] != "" {
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
				}
				return false
			},
			CreateFunc: func(evt event.CreateEvent) bool {
				if checkOwner(evt.Meta) {
					if _, isIngress := evt.Object.(*extensionsv1beta1.Ingress); isIngress {
						return true
					}
				}
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
	if rs != nil && rs.workspace != nil {
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
			if rs.wkspProps.started {
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
				rs.workspace.Status.WorkspaceId = rs.wkspProps.workspaceId
			}
		}

		log.Info("Status Update After Workspace Change : ", "status", rs.workspace.Status)
		err := r.Status().Update(context.Background(), rs.workspace)
		if err != nil {
			log.Error(err, "")
		}
	}
}

func (r *ReconcileWorkspace) updateStatusFromOwnedObjects(workspace *workspacev1alpha1.Workspace) (reconcile.Result, error) {
	workspace.Status.Members.Ready = []string {}
	workspace.Status.Members.Unready = []string {}
	for _, list := range []runtime.Object{
		&corev1.PodList{},
		&extensionsv1beta1.IngressList{},
	} {
		r.List(context.TODO(), &client.ListOptions{
			Namespace: workspace.GetNamespace(),
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"che.workspace_id": workspace.Status.WorkspaceId,
			}), // TODO Change this to look for objects owned by the workspace CR
		}, list)
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Addr().Interface()
			if itemPod, isPod := item.(*corev1.Pod); isPod {
				podOriginalName, originalNameFound := itemPod.GetLabels()["che.original_name"]
				if !originalNameFound {
					podOriginalName = "Unknown"
				}
				if _, podCondition := getPodCondition(&itemPod.Status, corev1.PodReady);
				podCondition != nil &&
				podCondition.Status == corev1.ConditionTrue {
					workspace.Status.Members.Ready = append(workspace.Status.Members.Ready, podOriginalName)
				} else {
					workspace.Status.Members.Unready = append(workspace.Status.Members.Unready, podOriginalName)
				}
				if podOriginalName == cheOriginalName {
					copyPodConditions(&itemPod.Status, &workspace.Status)
					clearCondition(&workspace.Status, workspacev1alpha1.WorkspaceConditionStopped)
					_, workspaceCondition := getWorkspaceCondition(&workspace.Status, workspacev1alpha1.WorkspaceConditionReady)
					if workspaceCondition != nil && workspaceCondition.Status == corev1.ConditionTrue {
						workspace.Status.Phase = workspacev1alpha1.WorkspacePhaseRunning
					}
				}
			}
			if itemIngress, isIngress := item.(*extensionsv1beta1.Ingress); isIngress {
				attributes := itemIngress.GetAnnotations()["org.eclipse.che.server.attributes"]
				if attributes != "" {
					attrs := map[string]string{}
					err := json.Unmarshal([]byte(attributes), &attrs)
					if err == nil && attrs["type"] == "ide" {						
						workspace.Status.IdeUrl = join("",
							itemIngress.GetAnnotations()["org.eclipse.che.server.protocol"],
							"://",
							itemIngress.Spec.Rules[0].Host)
					}
				}
			}
		}
	}
	if !workspace.Spec.Started {
		deploymentList := &appsv1.DeploymentList{}
		err := r.List(context.TODO(), &client.ListOptions{
			Namespace: workspace.GetNamespace(),
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"che.workspace_id": workspace.Status.WorkspaceId,
			}),
		}, deploymentList)
		if err == nil && len(deploymentList.Items) == 0 {
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
	if workspace.Status.Phase != workspacev1alpha1.WorkspacePhaseRunning {
		workspace.Status.IdeUrl = ""
	}
	log.Info("Status Update After Change To Owned Objects : ", "status", workspace.Status)
	r.Status().Update(context.Background(), workspace)
	return reconcile.Result{}, nil
}

var podConditionTypeToWorkspaceConditionType = map[corev1.PodConditionType]workspacev1alpha1.WorkspaceConditionType {
	corev1.PodScheduled: workspacev1alpha1.WorkspaceConditionScheduled,
	corev1.PodInitialized: workspacev1alpha1.WorkspaceConditionInitialized,
	corev1.PodReady: workspacev1alpha1.WorkspaceConditionReady,
}

func copyPodConditions(podStatus *corev1.PodStatus, workspaceStatus *workspacev1alpha1.WorkspaceStatus) {
	for _, podConditionType := range []corev1.PodConditionType {
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
		LastTransitionTime: metav1.Time {now},
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
