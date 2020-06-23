//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package registry

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/devfile/devworkspace-operator/internal/images"

	"github.com/eclipse/che-plugin-broker/model"
	brokerModel "github.com/eclipse/che-plugin-broker/model"
	"gopkg.in/yaml.v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// RegistryDirectory is the directory where the YAML files for the internal registry exist
	RegistryDirectory = "internal-registry/"
)

var log = logf.Log.WithName("registry")

// IsInInternalRegistry checks if pluginID is in the internal registry
func IsInInternalRegistry(pluginID string) bool {
	if _, err := os.Stat(RegistryDirectory + pluginID + "/meta.yaml"); err != nil {
		if os.IsNotExist(err) {
			log.Info(fmt.Sprintf("Could not find %s in the internal registry", pluginID))
		}
		return false
	}
	return true
}

// InternalRegistryPluginToMetaYAML converts a meta yaml coming from file path RegistryDirectory/pluginID/meta.yaml to PluginMeta
func InternalRegistryPluginToMetaYAML(pluginID string) (*brokerModel.PluginMeta, error) {
	yamlFile, err := ioutil.ReadFile(RegistryDirectory + pluginID + "/meta.yaml")
	if err != nil {
		return nil, err
	}

	var pluginMeta model.PluginMeta
	if err := yaml.Unmarshal(yamlFile, &pluginMeta); err != nil {
		return nil, fmt.Errorf(
			"failed to unmarshal downloaded meta.yaml for plugin '%s': %s", pluginID, err)
	}

	pluginMeta, err = images.FillPluginMetaEnvVars(pluginMeta)
	if err != nil {
		return nil, fmt.Errorf("could not process plugin %s from internal registry: %s", pluginID, err)
	}

	// Ensure ID field is set since it is used all over the place in broker
	// This could be unset if e.g. a meta.yaml is passed via a reference and does not have ID set.
	if pluginMeta.ID == "" {
		if pluginID != "" {
			pluginMeta.ID = pluginID
		} else {
			pluginMeta.ID = fmt.Sprintf("%s/%s/%s", pluginMeta.Publisher, pluginMeta.Name, pluginMeta.Version)
		}
	}
	return &pluginMeta, nil
}
