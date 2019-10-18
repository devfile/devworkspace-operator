package workspace

import (
	"fmt"
	"github.com/go-logr/logr"
	"context"
	origLog "log"
	"reflect"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	brokerCfg "github.com/eclipse/che-plugin-broker/cfg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_workspace")

var configMapReference = client.ObjectKey{
	Namespace: "",
	Name:      "che-workspace-crd-controller",
}

// Add creates a new Workspace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileWorkspace {
	return &ReconcileWorkspace{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileWorkspace) error {
	// Create a new controller
	c, err := controller.New("workspace-controller", mgr, controller.Options{
		Reconciler: r,
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return err
	}

	operatorNamespace, err := k8sutil.GetOperatorNamespace()
	if err == nil {
		configMapReference.Namespace = operatorNamespace
	} else if err != k8sutil.ErrNoNamespace {
		return err
	}

	err = watchControllerConfig(c, mgr)
	if err != nil {
		return err
	}

	if controllerConfig.getPluginRegistry() == "" {
		return fmt.Errorf("No Che plugin registry setup. To use the embedded registry, you should not run the operator locally.")
	}

	err = watchStatus(c, mgr)
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &workspacev1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaOld == nil {
				log.Error(nil, "UpdateEvent has no old metadata", "event", e)
				return false
			}
			if e.ObjectOld == nil {
				log.Error(nil, "GenericEvent has no old runtime object to update", "event", e)
				return false
			}
			if e.ObjectNew == nil {
				log.Error(nil, "GenericEvent has no new runtime object for update", "event", e)
				return false
			}
			if e.MetaNew == nil {
				log.Error(nil, "UpdateEvent has no new metadata", "event", e)
				return false
			}
			if e.MetaNew.GetGeneration() == e.MetaOld.GetGeneration() {
				return false
			}
			return true
		},
	})
	if err != nil {
		return err
	}

	brokerCfg.AuthEnabled = false
	brokerCfg.DisablePushingToEndpoint = true
	brokerCfg.UseLocalhostInPluginUrls = true
	brokerCfg.OnlyApplyMetadataActions = true

	origLog.SetOutput(r)

	isOS, err := IsOpenShift()
	if err != nil {
		return err
	}

	controllerConfig.controllerIsOpenshift = isOS

	return nil
}

func (r *ReconcileWorkspace) Write(p []byte) (n int, err error) {
	log.Info(string(p))
	return len(p), nil
}

var _ reconcile.Reconciler = &ReconcileWorkspace{}

// ReconcileWorkspace reconciles a Workspace object
type ReconcileWorkspace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme
}

type reconcileStatus struct {
	changedWorkspaceObjects   bool
	createdWorkspaceObjects   bool
	failure                   string
	cleanedWorkspaceObjects   bool
	wkspProps                 *workspaceProperties
	workspace                 *workspacev1alpha1.Workspace
	componentInstanceStatuses []ComponentInstanceStatus
	ReqLogger                 logr.Logger
}

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec&True
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWorkspace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reconcileStatus := &reconcileStatus{
		ReqLogger: reqLogger,
	}

	isStatusChange := false
	if strings.HasPrefix(request.Name, ownedObjectEventPrefix) {
		request.Name = strings.TrimPrefix(request.Name, ownedObjectEventPrefix)
		isStatusChange = true
	}

	reqLogger.V(1).Info("Reconciling")

	// Fetch the Workspace instance
	instance := &workspacev1alpha1.Workspace{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if isStatusChange {
		return r.updateStatusFromOwnedObjects(instance, reqLogger)
	}

	var workspaceProperties *workspaceProperties
	reconcileStatus.workspace = instance

	defer r.updateStatusAfterWorkspaceChange(reconcileStatus)

	prerequisites, err := managePrerequisites(instance)
	if err != nil {
		reconcileStatus.failure = err.Error()
		return reconcile.Result{}, err
	}

	reqLogger.Info("Managing K8s Pre-requisites")
	for _, prereq := range prerequisites {
		prereqAsMetaObject, isMeta := prereq.(metav1.Object)
		if !isMeta {
			reconcileStatus.failure = err.Error()
			return reconcile.Result{}, errors.NewBadRequest("Converted objects are not valid K8s objects")
		}

		reqLogger.V(1).Info("Managing K8s Pre-requisite", "kind", reflect.TypeOf(prereq).Elem().String(), "name", prereqAsMetaObject.GetName())

		found := reflect.New(reflect.TypeOf(prereq).Elem()).Interface().(runtime.Object)
		err = r.Get(context.TODO(), types.NamespacedName{Name: prereqAsMetaObject.GetName(), Namespace: prereqAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("    => Creating "+reflect.TypeOf(prereqAsMetaObject).Elem().String(), "namespace", prereqAsMetaObject.GetNamespace(), "name", prereqAsMetaObject.GetName())
			err = r.Create(context.TODO(), prereq)
			if err != nil {
				return reconcile.Result{}, err
			}
			continue
		} else if err != nil {
			reconcileStatus.failure = err.Error()
			return reconcile.Result{}, err
		} else {
			if _, isPVC := found.(*corev1.PersistentVolumeClaim); !isPVC {
				if _, isServiceAccount := found.(*corev1.ServiceAccount); !isServiceAccount {
					err = r.Update(context.TODO(), prereq)
					if err != nil {
						log.Error(err, "")
					}
				}
			}
		}
	}

	workspaceProperties, workspaceExposure, componentInstanceStatuses, k8sObjects, err := convertToCoreObjects(instance)
	reconcileStatus.wkspProps = workspaceProperties
	if err != nil {
		reqLogger.Error(err, "Error when converting to K8S objects")
		reconcileStatus.failure = err.Error()
		return reconcile.Result{}, nil
	}

	reconcileStatus.componentInstanceStatuses = componentInstanceStatuses
	k8sObjectNames := map[string]struct{}{}

	reqLogger.Info("Managing K8s Objects")
	for _, k8sObject := range append(k8sObjects, workspaceExposure) {
		k8sObjectAsMetaObject, isMeta := k8sObject.(metav1.Object)
		if !isMeta {
			return reconcile.Result{}, errors.NewBadRequest("Converted objects are not valid K8s objects")
		}
		k8sObjectNames[k8sObjectAsMetaObject.GetName()] = struct{}{}

		reqLogger.V(1).Info("  - Managing K8s Object", "kind", reflect.TypeOf(k8sObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())

		// Set Workspace instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, k8sObjectAsMetaObject, r.scheme); err != nil {
			reconcileStatus.failure = err.Error()
			reqLogger.Error(err, "Error when setting controller reference")
			return reconcile.Result{}, nil
		}

		k8sObjectAsMetaObject.SetLabels(map[string]string { "che.workspace_id": workspaceProperties.workspaceId })

		// Check if the k8s Object already exists

		found := reflect.New(reflect.TypeOf(k8sObject).Elem()).Interface().(runtime.Object)
		err = r.Get(context.TODO(), types.NamespacedName{Name: k8sObjectAsMetaObject.GetName(), Namespace: k8sObjectAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("  => Creating " + reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())
			err = r.Create(context.TODO(), k8sObject)
			if err != nil {
				reqLogger.Error(err, "Error when creating K8S object: ", "k8sObject", k8sObject)
				reconcileStatus.failure = err.Error()
				return reconcile.Result{}, nil
			}
			if deployment, isDeployment := k8sObject.(*appsv1.Deployment); isDeployment &&
				strings.HasSuffix(deployment.GetName(), "."+cheOriginalName) {
				reconcileStatus.createdWorkspaceObjects = true
			}
			continue
		} else if err != nil {
			reqLogger.Error(err, "Error when getting K8S object: ", "k8sObject", k8sObjectAsMetaObject)
			reconcileStatus.failure = err.Error()
			return reconcile.Result{}, nil
		}

		r.scheme.Default(k8sObject)

		// Update the found object and write the result back if there are any changes

		foundSpecValue := reflect.ValueOf(found).Elem().FieldByName("Spec")
		k8sObjectSpecValue := reflect.ValueOf(k8sObject).Elem().FieldByName("Spec")

		var foundToUse interface{} = found
		var newToUse interface{} = k8sObject
		if foundSpecValue.IsValid() {
			foundToUse = foundSpecValue.Interface()
			newToUse = k8sObjectSpecValue.Interface()
		}

		diffOpts := cmp.Options{
			cmpopts.IgnoreUnexported(resource.Quantity{}),
			cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP", "SessionAffinity", "Type"),
			cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy", "ImagePullPolicy"),
			cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SecurityContext", "SchedulerName", "DeprecatedServiceAccount", "RestartPolicy", "TerminationGracePeriodSeconds"),
			cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
			cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
			cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
			cmp.FilterPath(
				func(p cmp.Path) bool {
					s := p.String()
					return s == "Ports.Protocol"
				},
				cmp.Transformer("DefaultTcpProtocol", func(p corev1.Protocol) corev1.Protocol {
					if p == "" {
						return corev1.ProtocolTCP
					}
					return p
				})),
		}

		if !cmp.Equal(foundToUse, newToUse, diffOpts) {
			reqLogger.V(1).Info("  => Differences: " + cmp.Diff(foundToUse, newToUse, diffOpts...))
			switch found.(type) {
			case (*appsv1.Deployment):
				{
					found.(*appsv1.Deployment).Spec = k8sObject.(*appsv1.Deployment).Spec
					if strings.HasSuffix(found.(*appsv1.Deployment).GetName(), "."+cheOriginalName) {
						reconcileStatus.changedWorkspaceObjects = true
					}
				}
			case (*extensionsv1beta1.Ingress):
				{
					found.(*extensionsv1beta1.Ingress).Spec = k8sObject.(*extensionsv1beta1.Ingress).Spec
				}
			case (*corev1.Service):
				{
					k8sObject.(*corev1.Service).Spec.ClusterIP = found.(*corev1.Service).Spec.ClusterIP
					found.(*corev1.Service).Spec = k8sObject.(*corev1.Service).Spec
				}
			case (*corev1.ConfigMap):
				{
					found.(*corev1.ConfigMap).Data = k8sObject.(*corev1.ConfigMap).Data
					found.(*corev1.ConfigMap).BinaryData = k8sObject.(*corev1.ConfigMap).BinaryData
				}
			case (*workspacev1alpha1.WorkspaceExposure):
				{
					found.(*workspacev1alpha1.WorkspaceExposure).Spec = k8sObject.(*workspacev1alpha1.WorkspaceExposure).Spec
				}
			}
			reqLogger.Info("  => Updating " +reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())
			err = r.Update(context.TODO(), found)
			if err != nil {
				reqLogger.Error(err, "Error when updating K8S object: ", "k8sObject", k8sObjectAsMetaObject)
				reconcileStatus.failure = err.Error()
				return reconcile.Result{}, nil
			}
		}
	}

	if err != nil {
		reqLogger.Error(err, "Error during reconcile")
		reconcileStatus.failure = err.Error()
		return reconcile.Result{}, nil
	}

	for _, list := range []runtime.Object{
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&extensionsv1beta1.IngressList{},
		&corev1.ConfigMapList{},
	} {
		r.List(context.TODO(), &client.ListOptions{
			Namespace: workspaceProperties.namespace,
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"che.workspace_id": workspaceProperties.workspaceId,
			}),
		}, list)
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Addr().Interface()
			if itemMeta, isMeta := item.(metav1.Object); isMeta {
				if itemRuntime, isRuntime := item.(runtime.Object); isRuntime {
					if _, present := k8sObjectNames[itemMeta.GetName()]; !present {
						log.Info("  => Deleting "+reflect.TypeOf(itemRuntime).Elem().String(), "name", itemMeta.GetName())
						r.Delete(context.TODO(), itemRuntime)
						if _, isDeployment := itemRuntime.(*appsv1.Deployment); isDeployment &&
							strings.HasSuffix(itemMeta.GetName(), "."+cheOriginalName) {
							reconcileStatus.cleanedWorkspaceObjects = true
						}
					}
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
