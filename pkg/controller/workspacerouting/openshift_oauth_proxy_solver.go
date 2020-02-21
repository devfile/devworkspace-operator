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
	"github.com/che-incubator/che-workspace-operator/pkg/specutils"
	"k8s.io/apimachinery/pkg/api/resource"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routeV1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenshiftOAuthSolver struct {
	Client client.Client
}

func (solver *OpenshiftOAuthSolver) CreateDiscoverableServices(cr CurrentReconcile) []corev1.Service {
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

func (solver *OpenshiftOAuthSolver) CreateRoutes(cr CurrentReconcile) []runtime.Object {
	objectsToCreate := []runtime.Object{}

	currentInstance := cr.Instance

	// TODO: Temp workaround -- should be able to get serviceAcct separately.
	proxyDeployment, proxySA := specutils.GetProxyDeployment(currentInstance.Name, currentInstance.Namespace, currentInstance.Spec.Services)
	objectsToCreate = append(objectsToCreate, &proxyDeployment, &proxySA)

	proxyService := createServiceForContainerPorts(currentInstance.Name, currentInstance.Namespace, proxyDeployment)
	objectsToCreate = append(objectsToCreate, &proxyService)

	proxyRoutes := createRoutesForServicePorts(currentInstance.Namespace, currentInstance.Spec.IngressGlobalDomain, proxyService)
	for _, proxyRoute := range proxyRoutes {
		objectsToCreate = append(objectsToCreate, &proxyRoute)
	}

	for _, serviceDesc := range currentInstance.Spec.Services {
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] != "true" {
				continue
			}

			var tls *routeV1.TLSConfig = nil
			// TODO: Figure out why this is done?
			if endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" {
				if endpoint.Attributes[workspacev1alpha1.TYPE_ENDPOINT_ATTRIBUTE] == "terminal" {
					tls = &routeV1.TLSConfig{
						Termination:                   routeV1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
					}
				} else {
					continue
				}
			}

			objectsToCreate = append(objectsToCreate, &routeV1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      specutils.IngressName(serviceDesc.ServiceName, endpoint.Port),
					Namespace: currentInstance.Namespace,
				},
				Spec: routeV1.RouteSpec{
					Host: specutils.IngressHostname(
						serviceDesc.ServiceName,
						currentInstance.Namespace,
						currentInstance.Spec.IngressGlobalDomain,
						endpoint.Port),
					To: routeV1.RouteTargetReference{
						Kind: "Service",
						Name: serviceDesc.ServiceName,
					},
					Port: &routeV1.RoutePort{
						TargetPort: intstr.FromString(specutils.ServicePortName(int(endpoint.Port))),
					},
					TLS: tls,
				},
			})
		}
	}
	return objectsToCreate
}

func createServiceForContainerPorts(workspaceroutingname, namespace string, proxyDeployment appsv1.Deployment) corev1.Service {
	var servicePorts []corev1.ServicePort
	for _, container := range proxyDeployment.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:     port.Name,
				Port:     port.ContainerPort,
				Protocol: corev1.ProtocolTCP,
			})
		}
	}
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-workspace-proxy", // TODO : What should this be?
			Namespace: namespace,
			Annotations: map[string]string{
				// TODO
				//Annotations: map[string]string{
				//       "service.alpha.openshift.io/serving-cert-secret-name": "proxy-tls" + proxyCountString,
				//},
				"service.alpha.openshift.io/serving-cert-secret-name": "proxy-tls",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": specutils.ProxyDeploymentName(workspaceroutingname),
			},
			Type:  corev1.ServiceTypeClusterIP,
			Ports: servicePorts,
		},
	}

	return service
}

func createRoutesForServicePorts(namespace, ingressGlobalDomain string, service corev1.Service) []routeV1.Route {
	var routes []routeV1.Route

	for _, port := range service.Spec.Ports {
		portNum := int64(port.Port)
		servicePort := intstr.FromString(port.Name)
		route := routeV1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      specutils.IngressName(service.Name, portNum),
				Namespace: namespace,
			},
			Spec: routeV1.RouteSpec{
				Host: specutils.IngressHostname(service.Name, namespace, ingressGlobalDomain, portNum),
				To: routeV1.RouteTargetReference{
					Kind: "Service",
					Name: service.Name,
				},
				Port: &routeV1.RoutePort{
					TargetPort: servicePort,
				},
				TLS: &routeV1.TLSConfig{
					Termination:                   routeV1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		routes = append(routes, route)
	}

	return routes
}

func (solver *OpenshiftOAuthSolver) CreateOrUpdateRoutingObjects(cr CurrentReconcile) (reconcile.Result, error) {
	k8sObjects := []runtime.Object{}

	for _, discoverableService := range solver.CreateDiscoverableServices(cr) {
		newService := discoverableService
		k8sObjects = append(k8sObjects, &newService)
	}

	k8sObjects = append(k8sObjects, solver.CreateRoutes(cr)...)

	return CreateOrUpdate(cr, k8sObjects,
		cmp.Options{
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
		},
		func(found runtime.Object, new runtime.Object) {
			switch found.(type) {
			case (*routeV1.Route):
				{
					found.(*routeV1.Route).Spec = new.(*routeV1.Route).Spec
				}
			case (*corev1.Service):
				{
					new.(*corev1.Service).Spec.ClusterIP = found.(*corev1.Service).Spec.ClusterIP
					found.(*corev1.Service).Spec = new.(*corev1.Service).Spec
				}
			case (*appsv1.Deployment):
				{
					found.(*appsv1.Deployment).Spec = new.(*appsv1.Deployment).Spec
				}
			}
		},
	)
}

func (solver *OpenshiftOAuthSolver) CheckRoutingObjects(cr CurrentReconcile, targetPhase workspacev1alpha1.WorkspaceRoutingPhase) (workspacev1alpha1.WorkspaceRoutingPhase, reconcile.Result, error) {
	return targetPhase, reconcile.Result{}, nil
}

func (solver *OpenshiftOAuthSolver) BuildExposedEndpoints(cr CurrentReconcile) map[string]workspacev1alpha1.ExposedEndpointList {
	exposedEndpoints := map[string]workspacev1alpha1.ExposedEndpointList{}

	for containerName, serviceDesc := range cr.Instance.Spec.Services {
		containerExposedEndpoints := []workspacev1alpha1.ExposedEndpoint{}
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] == "false" {
				continue
			}
			protocol := endpoint.Attributes[workspacev1alpha1.PROTOCOL_ENDPOINT_ATTRIBUTE]
			if endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" {
				protocol = protocol + "s"
			}
			exposedEndpoint := workspacev1alpha1.ExposedEndpoint{
				Attributes: endpoint.Attributes,
				Name:       endpoint.Name,
				Url: protocol + "://" + specutils.IngressHostname(
					serviceDesc.ServiceName,
					cr.Instance.Namespace,
					cr.Instance.Spec.IngressGlobalDomain,
					endpoint.Port),
			}
			containerExposedEndpoints = append(containerExposedEndpoints, exposedEndpoint)
		}
		exposedEndpoints[containerName] = containerExposedEndpoints
	}

	return exposedEndpoints
}

func (solver *OpenshiftOAuthSolver) DeleteRoutingObjects(cr CurrentReconcile) (reconcile.Result, error) {
	return DeleteRoutingObjects(cr, []runtime.Object{
		&corev1.ServiceList{},
		&routeV1.RouteList{},
		&appsv1.Deployment{},
	})
}
