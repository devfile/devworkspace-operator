package workspaces

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type CodeReady struct {
	rest *rest.Config
}

// NewCodeReady creates C, a workspaces used to expose common testing functions.
func NewWorkspaceClient() *CodeReady {
	h := &CodeReady{}
	return h
}

// Kube returns the clientset for Kubernetes upstream.
func (c *CodeReady) Kube() kubernetes.Interface {
	cfg, _ := config.GetConfig()
	client, _ := kubernetes.NewForConfig(cfg)
	return client
}
