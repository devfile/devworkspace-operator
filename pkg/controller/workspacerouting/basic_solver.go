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

package workspacerouting

import (
	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	modelutils "github.com/che-incubator/che-workspace-operator/pkg/controller/modelutils/k8s"
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

func (solver *BasicSolver) CreateIngresses(cr CurrentReconcile) []extensionsv1beta1.Ingress {
	ingresses := []extensionsv1beta1.Ingress{}
	for _, serviceDesc := range cr.Instance.Spec.Services {
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] == "true" {
				ingresses = append(ingresses, extensionsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      modelutils.IngressName(serviceDesc.ServiceName, endpoint.Port),
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
								Host: modelutils.IngressHostname(serviceDesc.ServiceName, cr.Instance.Namespace, endpoint.Port),
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

func (solver *BasicSolver) CreateOrUpdateRoutingObjects(cr CurrentReconcile) (reconcile.Result, error) {
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

func (solver *BasicSolver) CheckRoutingObjects(cr CurrentReconcile, targetPhase workspacev1alpha1.WorkspaceRoutingPhase) (workspacev1alpha1.WorkspaceRoutingPhase, reconcile.Result, error) {
	return targetPhase, reconcile.Result{}, nil
}

func (solver *BasicSolver) BuildExposedEndpoints(cr CurrentReconcile) map[string]workspacev1alpha1.ExposedEndpointList {
	exposedEndpoints := map[string]workspacev1alpha1.ExposedEndpointList{}

	for containerName, serviceDesc := range cr.Instance.Spec.Services {
		containerExposedEndpoints := []workspacev1alpha1.ExposedEndpoint{}
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] == "false" {
				continue
			}
			exposedEndpoint := workspacev1alpha1.ExposedEndpoint{
				Attributes: endpoint.Attributes,
				Name:       endpoint.Name,
				Url:        endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE] + "://" + modelutils.IngressHostname(serviceDesc.ServiceName, cr.Instance.Namespace, endpoint.Port),
			}
			containerExposedEndpoints = append(containerExposedEndpoints, exposedEndpoint)
		}
		exposedEndpoints[containerName] = containerExposedEndpoints
	}

	return exposedEndpoints
}

func (solver *BasicSolver) DeleteRoutingObjects(cr CurrentReconcile) (reconcile.Result, error) {
	return DeleteRoutingObjects(cr, []runtime.Object{
		&corev1.ServiceList{},
		&extensionsv1beta1.IngressList{},
	})
}
