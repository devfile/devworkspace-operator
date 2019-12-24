package workspace

import (
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func IsOpenShift() (bool, error) {
	kubeconfig, err := config.GetConfig()
	if err != nil {
		return false, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return false, err
	}
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, err
	}
	apiGroups := apiList.Groups
	for i := 0; i < len(apiGroups); i++ {
		if apiGroups[i].Name == "route.openshift.io" {
			return true, nil
		}
	}
	return false, nil
}
