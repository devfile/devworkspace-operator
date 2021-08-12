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

package config

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	wsprovision "github.com/devfile/devworkspace-operator/pkg/provision/workspace"
)

const (
	commonPVCSizeKey = "commonPVCSize"
)

type NamespacedConfig struct {
	CommonPVCSize string
}

// ReadNamespacedConfig reads the per-namespace DevWorkspace configmap and returns it as a struct. If there are
// no valid configmaps in the specified namespace, returns (nil, nil). If there are multiple configmaps with the
// per-namespace configmap label, returns an error.
func ReadNamespacedConfig(namespace string, api wsprovision.ClusterAPI) (*NamespacedConfig, error) {
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
		CommonPVCSize: cm.Data[commonPVCSizeKey],
	}, nil
}
