// Copyright (c) 2019-2025 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bootstrap

import (
	"fmt"
	"os"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func setupKubeClient() (client.Client, error) {
	scheme := k8sruntime.NewScheme()

	if err := dw.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to set up Kubernetes client: %w", err)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read in-cluster Kubernetes configuration: %w", err)
	}

	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return kubeClient, nil
}

func getWorkspaceNamespacedName() (types.NamespacedName, error) {
	name := os.Getenv(constants.DevWorkspaceName)
	namespace := os.Getenv(constants.DevWorkspaceNamespace)

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if name == "" || namespace == "" {
		return namespacedName, fmt.Errorf("could not get workspace name or namespace from environment variables")
	}
	return namespacedName, nil
}
