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

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/devfile/devworkspace-operator/cert-generation/config"
	"github.com/devfile/devworkspace-operator/cert-generation/webhooks"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	log.SetOutput(os.Stdout)

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed when attempting to retrieve in cluster config: ", err)
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatal("Failed when attempting to retrieve in cluster config: ", err)
	}

	namespaceByte, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		log.Fatal("Could not retrieve namespace: ", err)
	}

	namespace := string(namespaceByte)
	configMapData := make(map[string]string, 0)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.CertConfigMapName,
			Namespace: namespace,
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
		Data: configMapData,
	}

	// Create the configmap or update if it already exists
	if _, err := clientset.CoreV1().ConfigMaps(namespace).Get(config.CertConfigMapName, metav1.GetOptions{}); errors.IsNotFound(err) {
		_, err = clientset.CoreV1().ConfigMaps(namespace).Create(configMap)
		if err != nil {
			log.Fatal("Failed when attempting to create configmap: ", err)
		}
	} else {
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(configMap)
		if err != nil {
			log.Fatal("Failed when attempting to update configmap: ", err)
		}
	}

	label := map[string]string{"app": "che-workspace-controller"}

	port := int32(443)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SecureServiceName,
			Namespace: namespace,
			Labels:    label,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": config.CertSecretName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					Protocol:   "TCP",
					TargetPort: intstr.FromString(config.WebhookServerName),
				},
			},
			Selector: label,
		},
	}

	// Create secure service or update it if it already exists
	if clusterService, err := clientset.CoreV1().Services(namespace).Get(config.SecureServiceName, metav1.GetOptions{}); errors.IsNotFound(err) {
		_, err = clientset.CoreV1().Services(namespace).Create(service)
		if err != nil {
			log.Fatal("Failed when attempting to create service: ", err)
		}
	} else {
		// Cannot naively copy spec, as clusterIP is unmodifiable
		clusterIP := clusterService.Spec.ClusterIP
		service.Spec = clusterService.Spec
		service.Spec.ClusterIP = clusterIP
		service.ResourceVersion = clusterService.ResourceVersion

		_, err = clientset.CoreV1().Services(namespace).Update(service)
		if err != nil {
			log.Fatal("Failed when attempting to update service: ", err)
		}
	}

	err = webhooks.WebhookInit(clientset, namespace)
	if err != nil {
		log.Fatal("Failed when attempting to setup the webhook: ", err)
	}

	log.Println("Certificates and webhooks are up to date. Idling until interrupted")
	var shutdownChan = make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)
	<-shutdownChan
}
