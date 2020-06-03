package workspaces

import (
	"fmt"
	"io/ioutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func deserializePluginRegistryDeployment() (pluginRegistry *appsv1.Deployment, err error) {
	if err != nil {
		fmt.Println("Failed to locate operator deployment yaml, %s", err)
	}
	file, err := ioutil.ReadFile("deploy/registry/local/deployment.yaml")
	if err != nil {
		fmt.Println("Failed to locate operator deployment yaml, %s", err)
	}
	deployment := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(deployment), nil, nil)
	if err != nil {
		fmt.Println("Failed to deserialize yaml %s", err)
		return nil, err
	}
	pluginRegistry = object.(*appsv1.Deployment)
	return pluginRegistry, nil
}

func deserializePluginRegistryService() (pluginRegistry *corev1.Service, err error) {
	if err != nil {
		fmt.Println("Failed to locate operator deployment yaml, %s", err)
	}
	file, err := ioutil.ReadFile("deploy/registry/local/service.yaml")
	if err != nil {
		fmt.Println("Failed to locate operator deployment yaml, %s", err)
	}
	deployment := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(deployment), nil, nil)
	if err != nil {
		fmt.Println("Failed to deserialize yaml %s", err)
		return nil, err
	}
	pluginRegistry = object.(*corev1.Service)
	return pluginRegistry, nil
}
