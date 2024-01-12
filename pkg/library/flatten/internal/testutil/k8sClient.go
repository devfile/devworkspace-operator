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

package testutil

import (
	"context"
	"errors"
	"fmt"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeK8sClient struct {
	client.Client         // To satisfy interface; override all used methods
	DevWorkspaceResources map[string]dw.DevWorkspaceTemplate
	Errors                map[string]TestPluginError
}

func (client *FakeK8sClient) Get(_ context.Context, namespacedName client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	template, ok := obj.(*dw.DevWorkspaceTemplate)
	if !ok {
		return fmt.Errorf("called Get() in fake client with non-DevWorkspaceTemplate")
	}
	if plugin, ok := client.DevWorkspaceResources[namespacedName.Name]; ok {
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
