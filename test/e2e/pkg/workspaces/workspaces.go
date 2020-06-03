package workspaces

import (
	"fmt"

	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/metadata"
)

func (w *CodeReady) CreatePluginRegistryDeployment() (err error) {
	label := "app=che-plugin-registry"
	deployment, err := deserializePluginRegistryDeployment()

	if err != nil {
		fmt.Println("Failed to deserialize deployment")
		return err
	}

	deployment, err = w.Kube().AppsV1().Deployments(metadata.Namespace.Name).Create(deployment)

	if err != nil {
		fmt.Println("Failed to create deployment %s: %s", deployment.Name, err)
		return err
	}

	deploy, err := w.PodDeployWaitUtil(label)
	if !deploy {
		fmt.Println("Che Workspaces Controller not deployed")
		return err
	}
	return nil
}

func (w *CodeReady) CreatePluginRegistryService() (err error) {
	deserializeService, _ := deserializePluginRegistryService()

	service, err := w.Kube().CoreV1().Services(metadata.Namespace.Name).Create(deserializeService)
	if err != nil {
		fmt.Println("Failed to create deployment %s: %s", service.Name, err)
		return err
	}
	return nil
}
