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

package testutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeK8sClient struct {
	client.Client // To satisfy interface; override all used methods
	Plugins       map[string]v1alpha2.DevWorkspaceTemplate
	Errors        map[string]TestPluginError
}

func (client *FakeK8sClient) Get(_ context.Context, namespacedName client.ObjectKey, obj runtime.Object) error {
	template, ok := obj.(*v1alpha2.DevWorkspaceTemplate)
	if !ok {
		return fmt.Errorf("called Get() in fake client with non-DevWorkspaceTemplate")
	}
	if plugin, ok := client.Plugins[namespacedName.Name]; ok {
		*template = plugin
		return nil
	}
	if err, ok := client.Errors[namespacedName.Name]; ok {
		if err.IsNotFound {
			return k8sErrors.NewNotFound(schema.GroupResource{}, namespacedName.Name)
		} else {
			return errors.New(err.Message)
		}
	}
	return fmt.Errorf("test does not define an entry for plugin %s", namespacedName.Name)
}
