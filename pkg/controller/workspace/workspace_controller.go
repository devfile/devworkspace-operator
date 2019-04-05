package workspace

import (
	"context"
	"reflect"

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
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWorkspace{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("workspace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	operatorNamespace, err := k8sutil.GetOperatorNamespace()
	if err == nil {
		configMapReference.Namespace = operatorNamespace
	} else if err != k8sutil.ErrNoNamespace {
		return err
	} else {
		watchNamespace, err := k8sutil.GetWatchNamespace()
		if err != nil {
			return err
		}
		configMapReference.Namespace = watchNamespace
	}

	err = watchWorkspaceConfig(c, mgr)
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &workspacev1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	brokerCfg.AuthEnabled = false
	brokerCfg.DisablePushingToEndpoint = true
	brokerCfg.UseLocalhostInPluginUrls = true
	brokerCfg.OnlyApplyMetadataActions = true

	/*
		for _, obj := range []runtime.Object{
			&appsv1.Deployment{},
			&corev1.Service{},
			&extensionsv1beta1.Ingress{},
		} {
			err = c.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &workspacev1alpha1.Workspace{},
			})
			if err != nil {
				return err
			}
		}
	*/
	return nil
}

var _ reconcile.Reconciler = &ReconcileWorkspace{}

// ReconcileWorkspace reconciles a Workspace object
type ReconcileWorkspace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWorkspace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Workspace")

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

	// If started == false => delete add services + ingresses,

	prerequisites, err := managePrerequisites(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, prereq := range prerequisites {
		prereqAsMetaObject, isMeta := prereq.(metav1.Object)
		if !isMeta {
			return reconcile.Result{}, errors.NewBadRequest("Converted objects are not valid K8s objects")
		}

		reqLogger.Info("Managing K8s Pre-requisite Object", "namespace", prereqAsMetaObject.GetNamespace(), "kind", reflect.TypeOf(prereq).Elem().String(), "name", prereqAsMetaObject.GetName())

		found := reflect.New(reflect.TypeOf(prereq).Elem()).Interface().(runtime.Object)
		err = r.Get(context.TODO(), types.NamespacedName{Name: prereqAsMetaObject.GetName(), Namespace: prereqAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("    => Creating "+reflect.TypeOf(prereqAsMetaObject).Elem().String(), "namespace", prereqAsMetaObject.GetNamespace(), "name", prereqAsMetaObject.GetName())
			err = r.Create(context.TODO(), prereq)
			continue
		} else if err != nil {
			return reconcile.Result{}, err
		} else {
			err = r.Update(context.TODO(), prereq)
		}
	}

	workspaceProperties, k8sObjects, err := convertToCoreObjects(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	k8sObjectNames := map[string]struct{}{}

	for _, k8sObject := range k8sObjects {
		k8sObjectAsMetaObject, isMeta := k8sObject.(metav1.Object)
		if !isMeta {
			return reconcile.Result{}, errors.NewBadRequest("Converted objects are not valid K8s objects")
		}
		k8sObjectNames[k8sObjectAsMetaObject.GetName()] = struct{}{}

		reqLogger.Info("Managing K8s Object", "namespace", k8sObjectAsMetaObject.GetNamespace(), "kind", reflect.TypeOf(k8sObject).Elem().String(), "name", k8sObjectAsMetaObject.GetName())

		// Set Workspace instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, k8sObjectAsMetaObject, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if the k8s Object already exists

		found := reflect.New(reflect.TypeOf(k8sObject).Elem()).Interface().(runtime.Object)
		err = r.Get(context.TODO(), types.NamespacedName{Name: k8sObjectAsMetaObject.GetName(), Namespace: k8sObjectAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("    => Creating "+reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "namespace", k8sObjectAsMetaObject.GetNamespace(), "name", k8sObjectAsMetaObject.GetName())
			err = r.Create(context.TODO(), k8sObject)
			if err != nil {
				return reconcile.Result{}, err
			}
			continue
		} else if err != nil {
			return reconcile.Result{}, err
		}

		// Update the found object and write the result back if there are any changes
		foundSpecValue := reflect.ValueOf(k8sObject).Elem().FieldByName("Spec")
		k8sObjectSpecValue := reflect.ValueOf(found).Elem().FieldByName("Spec")
		foundSpec := foundSpecValue.Interface()
		k8sObjectSpec := k8sObjectSpecValue.Interface()
		diffOpts := cmp.Options{
			cmpopts.IgnoreUnexported(resource.Quantity{}),
			cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP", "SessionAffinity"),
			cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
			cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SecurityContext", "SchedulerName"),
			cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
			cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
		}
		reqLogger.Info("    => Differences: " + cmp.Diff(k8sObjectSpec, foundSpec, diffOpts...))

		if !cmp.Equal(k8sObjectSpec, foundSpec, diffOpts) {
			switch found.(type) {
			case (*appsv1.Deployment):
				{
					(found).(*appsv1.Deployment).Spec = (k8sObject).(*appsv1.Deployment).Spec
				}
			case (*extensionsv1beta1.Ingress):
				{
					(found).(*extensionsv1beta1.Ingress).Spec = (k8sObject).(*extensionsv1beta1.Ingress).Spec
				}
			case (*corev1.Service):
				{
					(k8sObject).(*corev1.Service).Spec.ClusterIP = (found).(*corev1.Service).Spec.ClusterIP
					(found).(*corev1.Service).Spec = (k8sObject).(*corev1.Service).Spec
				}
			}
			reqLogger.Info("    => Updating "+reflect.TypeOf(k8sObjectAsMetaObject).Elem().String(), "namespace", k8sObjectAsMetaObject.GetNamespace(), "name", k8sObjectAsMetaObject.GetName())
			err = r.Update(context.TODO(), found)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if err != nil {
		return reconcile.Result{}, err
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
			}), // TODO Change this to look for objects owned by the workspace CR
		}, list)
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Addr().Interface()
			if itemMeta, isMeta := item.(metav1.Object); isMeta {
				if itemRuntime, isRuntime := item.(runtime.Object); isRuntime {
					if _, present := k8sObjectNames[itemMeta.GetName()]; !present {
						log.Info("    => Deleting "+reflect.TypeOf(itemRuntime).Elem().String(), "namespace", itemMeta.GetNamespace(), "name", itemMeta.GetName())
						r.Delete(context.TODO(), itemRuntime)
					}
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
