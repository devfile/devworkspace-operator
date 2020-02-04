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
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
	"k8s.io/apimachinery/pkg/api/resource"
	"strconv"

	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/config"
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

func routeName(serviceDesc workspacev1alpha1.ServiceDescription, endpoint workspacev1alpha1.Endpoint) string {
	portString := strconv.FormatInt(endpoint.Port, 10)
	return serviceDesc.ServiceName + "-" + portString
}

func routeHost(serviceDesc workspacev1alpha1.ServiceDescription, endpoint workspacev1alpha1.Endpoint, routing *workspacev1alpha1.WorkspaceRouting) string {
	return routeName(serviceDesc, endpoint) + "-" + routing.Namespace + "." + routing.Spec.IngressGlobalDomain
}

func proxyServiceAccountName(routing *workspacev1alpha1.WorkspaceRouting) string {
	return routing.Name + "-oauth-proxy"
}

func proxyDeploymentName(routing *workspacev1alpha1.WorkspaceRouting) string {
	return routing.Name + "-oauth-proxy"
}

func proxyServiceName(serviceDesc workspacev1alpha1.ServiceDescription, endpoint workspacev1alpha1.Endpoint) string {
	return routeName(serviceDesc, endpoint) + "-oauth-proxy"
}

func (solver *OpenshiftOAuthSolver) CreateRoutes(cr CurrentReconcile) []runtime.Object {
	objectsToCreate := []runtime.Object{}

	proxyServiceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        proxyServiceAccountName(cr.Instance),
			Namespace:   cr.Instance.Namespace,
			Annotations: map[string]string{},
		},
	}

	objectsToCreate = append(objectsToCreate, &proxyServiceAccount)

	var replicas int32 = 1
	proxyDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyDeploymentName(cr.Instance),
			Namespace: cr.Instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": proxyDeploymentName(cr.Instance),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": proxyDeploymentName(cr.Instance),
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: proxyServiceAccountName(cr.Instance),
					Volumes:            []corev1.Volume{},
					Containers:         []corev1.Container{},
				},
			},
		},
	}

	objectsToCreate = append(objectsToCreate, &proxyDeployment)

	initialProxyHttpPort := 4180
	initialProxyHttpsPort := 8443

	proxyCount := 0
	for _, serviceDesc := range cr.Instance.Spec.Services {
		for _, endpoint := range serviceDesc.Endpoints {
			if endpoint.Attributes[workspacev1alpha1.PUBLIC_ENDPOINT_ATTRIBUTE] == "true" {
				proxyCountString := strconv.FormatInt(int64(proxyCount), 10)
				targetServiceName := serviceDesc.ServiceName
				targetServicePort := k8sModelUtils.ServicePortName(int(endpoint.Port))
				var tls *routeV1.TLSConfig = nil

				if endpoint.Attributes[workspacev1alpha1.SECURE_ENDPOINT_ATTRIBUTE] == "true" {
					if endpoint.Attributes[workspacev1alpha1.TYPE_ENDPOINT_ATTRIBUTE] == "terminal" {
						tls = &routeV1.TLSConfig{
							Termination:                   routeV1.TLSTerminationEdge,
							InsecureEdgeTerminationPolicy: routeV1.InsecureEdgeTerminationPolicyRedirect,
						}
					} else {
						targetServiceName = proxyServiceName(serviceDesc, endpoint)
						targetServicePort = "proxy"
						tls = &routeV1.TLSConfig{
							Termination: routeV1.TLSTerminationReencrypt,
						}

						proxyHttpPort := initialProxyHttpPort + proxyCount
						proxyHttpPortString := strconv.FormatInt(int64(proxyHttpPort), 10)
						proxyHttpsPort := initialProxyHttpsPort + proxyCount
						proxyHttpsPortString := strconv.FormatInt(int64(proxyHttpsPort), 10)
						targetPortString := strconv.FormatInt(int64(endpoint.Port), 10)

						proxyDeployment.Spec.Template.Spec.Containers = append(proxyDeployment.Spec.Template.Spec.Containers, corev1.Container{
							Name: "oauth-proxy-" + proxyCountString,
							Ports: []corev1.ContainerPort{
								{
									Name:          "public",
									ContainerPort: int32(proxyHttpsPort),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ImagePullPolicy: corev1.PullPolicy(ControllerCfg.GetSidecarPullPolicy()),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "proxy-tls" + proxyCountString,
									MountPath: "/etc/tls/private",
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							Image:                    "openshift/oauth-proxy:latest",
							Args: []string{
								"--https-address=:" + proxyHttpsPortString,
								"--http-address=127.0.0.1:" + proxyHttpPortString,
								"--provider=openshift",
								"--openshift-service-account=" + proxyServiceAccountName(cr.Instance),
								"--upstream=http://" + serviceDesc.ServiceName + ":" + targetPortString,
								"--tls-cert=/etc/tls/private/tls.crt",
								"--tls-key=/etc/tls/private/tls.key",
								"--cookie-secret=SECRET",
							},
						})

						var volumeDefaultMode int32 = 420
						proxyDeployment.Spec.Template.Spec.Volumes = append(proxyDeployment.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "proxy-tls" + proxyCountString,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "proxy-tls" + proxyCountString,
									DefaultMode: &volumeDefaultMode,
								},
							},
						})

						proxyServiceAccount.Annotations["serviceaccounts.openshift.io/oauth-redirectreference."+proxyCountString] =
							`{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"` +
								routeName(serviceDesc, endpoint) +
								`"}}`

						objectsToCreate = append(objectsToCreate, &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name:      targetServiceName,
								Namespace: cr.Instance.Namespace,
								Annotations: map[string]string{
									"service.alpha.openshift.io/serving-cert-secret-name": "proxy-tls" + proxyCountString,
								},
							},
							Spec: corev1.ServiceSpec{
								Selector: map[string]string{
									"app": proxyDeploymentName(cr.Instance),
								},
								Type: corev1.ServiceTypeClusterIP,
								Ports: []corev1.ServicePort{
									corev1.ServicePort{
										Name:     "proxy",
										Port:     int32(proxyHttpsPort),
										Protocol: corev1.ProtocolTCP,
									},
								},
							},
						})
						proxyCount++
					}
				}

				objectsToCreate = append(objectsToCreate, &routeV1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName(serviceDesc, endpoint),
						Namespace: cr.Instance.Namespace,
					},
					Spec: routeV1.RouteSpec{
						Host: routeHost(serviceDesc, endpoint, cr.Instance),
						To: routeV1.RouteTargetReference{
							Kind: "Service",
							Name: targetServiceName,
						},
						Port: &routeV1.RoutePort{
							TargetPort: intstr.FromString(targetServicePort),
						},
						TLS: tls,
					},
				})
			}
		}
	}
	return objectsToCreate
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
				Url:        protocol + "://" + routeHost(serviceDesc, endpoint, cr.Instance),
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
