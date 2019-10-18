package workspaceexposure

import (
	"strconv"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type BasicSolver struct {
	Client client.Client
}

func (solver *BasicSolver) CreateDiscoverableServices(cr CurrentReconcile) []corev1.Service {
	discoverableServices := []corev1.Service{}
	for _, serviceDesc := range cr.Instance.Spec.Services {
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes["discoverable"] == "true" {
				discoverableServices = append(discoverableServices, corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      endpoint.Name,
						Namespace: cr.Instance.Namespace,
					},
					Spec: corev1.ServiceSpec{
						Selector: cr.Instance.Spec.WorkspacePodSelector,
						Type:     corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Port:     int32(endpoint.Port),
								Protocol: corev1.ProtocolTCP,
							},
						},
					},
				})
			}
		}
	}
	return discoverableServices
}

func ingressName(serviceDesc workspacev1alpha1.ServiceDescription, endpoint workspacev1alpha1.Endpoint) string {
	portString := strconv.FormatInt(endpoint.Port, 10)
	return serviceDesc.ServiceName + "-" + portString
}

func ingressHost(serviceDesc workspacev1alpha1.ServiceDescription, endpoint workspacev1alpha1.Endpoint, exposure *workspacev1alpha1.WorkspaceExposure) string {
	return ingressName(serviceDesc, endpoint) + "-" + exposure.Namespace + "." + exposure.Spec.IngressGlobalDomain
}

func (solver *BasicSolver) CreateIngresses(cr CurrentReconcile) []extensionsv1beta1.Ingress {
	ingresses := []extensionsv1beta1.Ingress{}
	for _, serviceDesc := range cr.Instance.Spec.Services {
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes["public"] == "true" {
				ingresses = append(ingresses, extensionsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ingressName(serviceDesc, endpoint),
						Namespace: cr.Instance.Namespace,
						Annotations: map[string]string{
							"kubernetes.io/ingress.class":                "nginx",
							"nginx.ingress.kubernetes.io/rewrite-target": "/",
							"nginx.ingress.kubernetes.io/ssl-redirect":   "false",
						},
					},
					Spec: extensionsv1beta1.IngressSpec{
						Rules: []extensionsv1beta1.IngressRule{
							extensionsv1beta1.IngressRule{
								Host: ingressHost(serviceDesc, endpoint, cr.Instance),
								IngressRuleValue: extensionsv1beta1.IngressRuleValue{
									HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
										Paths: []extensionsv1beta1.HTTPIngressPath{
											extensionsv1beta1.HTTPIngressPath{
												Backend: extensionsv1beta1.IngressBackend{
													ServiceName: serviceDesc.ServiceName,
													ServicePort: intstr.FromInt(int(endpoint.Port)),
												},
											},
										},
									},
								},
							},
						},
					},
				})
			}
		}
	}
	return ingresses
}

func (solver *BasicSolver) CreateOrUpdateExposureObjects(cr CurrentReconcile) (reconcile.Result, error) {
	k8sObjects := []runtime.Object{}

	for _, discoverableService := range solver.CreateDiscoverableServices(cr) {
		newService := discoverableService
		k8sObjects = append(k8sObjects, &newService)
	}

	for _, ingress := range solver.CreateIngresses(cr) {
		newIngress := ingress
		k8sObjects = append(k8sObjects, &newIngress)
	}

	return CreateOrUpdate(cr, k8sObjects,
		cmp.Options{
			cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP", "SessionAffinity", "Type"),
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
		},
		func(found runtime.Object, new runtime.Object) {
			switch found.(type) {
			case (*extensionsv1beta1.Ingress):
				{
					found.(*extensionsv1beta1.Ingress).Spec = new.(*extensionsv1beta1.Ingress).Spec
				}
			case (*corev1.Service):
				{
					new.(*corev1.Service).Spec.ClusterIP = found.(*corev1.Service).Spec.ClusterIP
					found.(*corev1.Service).Spec = new.(*corev1.Service).Spec
				}
			}
		},
	)

	return reconcile.Result{}, nil
}

func (solver *BasicSolver) CheckExposureObjects(cr CurrentReconcile, targetPhase workspacev1alpha1.WorkspaceExposurePhase) (workspacev1alpha1.WorkspaceExposurePhase, reconcile.Result, error) {
	return targetPhase, reconcile.Result{}, nil
}

func (solver *BasicSolver) BuildExposedEndpoints(cr CurrentReconcile) map[string][]workspacev1alpha1.ExposedEndpoint {
	exposedEndpoints := map[string][]workspacev1alpha1.ExposedEndpoint{}

	for machineName, serviceDesc := range cr.Instance.Spec.Services {
		machineExposedEndpoints := []workspacev1alpha1.ExposedEndpoint{}
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes["public"] == "false" {
				continue
			}
			exposedEndpoint := workspacev1alpha1.ExposedEndpoint{
				Attributes: endpoint.Attributes,
				Name:       endpoint.Name,
				Url:        endpoint.Attributes["protocol"] + "://" + ingressHost(serviceDesc, endpoint, cr.Instance),
			}
			machineExposedEndpoints = append(machineExposedEndpoints, exposedEndpoint)
		}
		exposedEndpoints[machineName] = machineExposedEndpoints
	}

	return exposedEndpoints
}

func (solver *BasicSolver) DeleteExposureObjects(cr CurrentReconcile) (reconcile.Result, error) {
	return DeleteExposureObjects(cr, []runtime.Object{
		&corev1.ServiceList{},
		&extensionsv1beta1.IngressList{},
	})
}
