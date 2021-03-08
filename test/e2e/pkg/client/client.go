//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package client

import (
	"fmt"

	devworkspacev1alpha2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(devworkspacev1alpha2.AddToScheme(scheme))
}

type K8sClient struct {
	kubeClient  *kubernetes.Clientset
	crClient    crclient.Client
	kubeCfgFile string // generate when client is created and store config there
}

// NewK8sClientWithKubeConfig creates kubernetes client wrapper with the specified kubeconfig file
func NewK8sClientWithKubeConfig(kubeconfigFile string) (*K8sClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, err
	}

	cfgBump := fmt.Sprintf("/tmp/admin.%s.kubeconfig", generateUniqPrefixForFile())
	err = copyFile(kubeconfigFile, cfgBump)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(cfg, crclient.Options{})
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		kubeClient:  client,
		crClient:    crClient,
		kubeCfgFile: cfgBump,
	}, nil
}

// NewK8sClientWithKubeConfig creates kubernetes client wrapper with the token
func NewK8sClientWithToken(baseKubeConfig, token string) (*K8sClient, error) {
	cfgBump := fmt.Sprintf("/tmp/dev.%s.kubeconfig", generateUniqPrefixForFile())
	err := copyFile(baseKubeConfig, cfgBump)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("bash",
		"-c", fmt.Sprintf(
			"KUBECONFIG=%s"+
				" oc login --token %s --insecure-skip-tls-verify=true",
			cfgBump, token))
	outBytes, err := cmd.CombinedOutput()
	output := string(outBytes)
	cfg, err := clientcmd.BuildConfigFromFlags("", cfgBump)
	if err != nil {
		log.Printf("Failed to login with oc: %s", output)
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crClient, err := crclient.New(cfg, crclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kubeClient:  client,
		crClient:    crClient,
		kubeCfgFile: cfgBump,
	}, nil
}

// Kube returns the clientset for Kubernetes upstream.
func (c *K8sClient) Kube() kubernetes.Interface {
	return c.kubeClient
}

//read a source file and copy to the selected path
func copyFile(sourceFile string, destinationFile string) error {
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(destinationFile, input, 0644)
	if err != nil {
		return err
	}
	return nil
}

//generateUniqPrefixForFile generates unique prefix by using current time in milliseconds and get last 5 numbers
func generateUniqPrefixForFile() string {
	//get the uniq time in seconds as string
	prefix := strconv.FormatInt(time.Now().UnixNano(), 10)
	//cut the string to last 5 uniq numbers
	prefix = prefix[14:]
	return prefix
}
