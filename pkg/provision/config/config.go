//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
//

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
)

const (
	commonPVCSizeKey       = "commonPVCSize"
	perWorkspacePVCSizeKey = "perWorkspacePVCSize"
)

type NamespacedConfig struct {
	CommonPVCSize       string
	PerWorkspacePVCSize string
}

// ReadNamespacedConfig reads the per-namespace DevWorkspace configmap and returns it as a struct. If there are
// no valid configmaps in the specified namespace, returns (nil, nil). If there are multiple configmaps with the
// per-namespace configmap label, returns an error.
func ReadNamespacedConfig(namespace string, api sync.ClusterAPI) (*NamespacedConfig, error) {
	cmList := &corev1.ConfigMapList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=true", constants.NamespacedConfigLabelKey))
	if err != nil {
		return nil, err
	}
	selector := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}
	err = api.Client.List(api.Ctx, cmList, selector)
	if err != nil {
		return nil, err
	}
	cms := cmList.Items
	if len(cms) == 0 {
		return nil, nil
	} else if len(cms) > 1 {
		var cmNames []string
		for _, cm := range cms {
			cmNames = append(cmNames, cm.Name)
		}
		return nil, fmt.Errorf("multiple per-namespace configs found: %s", strings.Join(cmNames, ", "))
	}

	cm := cms[0]
	if cm.Data == nil {
		return nil, nil
	}

	return &NamespacedConfig{
		CommonPVCSize:       cm.Data[commonPVCSizeKey],
		PerWorkspacePVCSize: cm.Data[perWorkspacePVCSizeKey],
	}, nil
}

// GetNamespacePodTolerationsAndNodeSelector gets pod tolerations and the node selector that should be applied to all pods created
// for workspaces in a given namespace. Tolerations and node selector are unmarshalled from json-formatted annotations on the namespace
// itself. Returns an error if annotations are not valid JSON.
func GetNamespacePodTolerationsAndNodeSelector(namespace string, api sync.ClusterAPI) ([]corev1.Toleration, map[string]string, error) {
	ns := &corev1.Namespace{}
	err := api.Client.Get(api.Ctx, types.NamespacedName{Name: namespace}, ns)
	if err != nil {
		return nil, nil, err
	}

	var podTolerations []corev1.Toleration
	podTolerationsAnnot, ok := ns.Annotations[constants.NamespacePodTolerationsAnnotation]
	if ok && podTolerationsAnnot != "" {
		if err := json.Unmarshal([]byte(podTolerationsAnnot), &podTolerations); err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s annotation: %w", constants.NamespacePodTolerationsAnnotation, err)
		}
	}

	nodeSelector := map[string]string{}
	nodeSelectorAnnot, ok := ns.Annotations[constants.NamespaceNodeSelectorAnnotation]
	if ok && nodeSelectorAnnot != "" {
		if err := json.Unmarshal([]byte(nodeSelectorAnnot), &nodeSelector); err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s annotation: %w", constants.NamespaceNodeSelectorAnnotation, err)
		}
	}

	return podTolerations, nodeSelector, nil
}
