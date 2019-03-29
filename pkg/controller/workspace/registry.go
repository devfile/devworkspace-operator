//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspace

import (
	"errors"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func getPluginMeta(registryUrl string, pluginCompositeId string) (*model.PluginMeta, error) {

	repo := registryUrl + "/plugins"
	var idAndVersion string
	if repoEnd := strings.LastIndex(pluginCompositeId, "/"); repoEnd >= 0 {
		repo = pluginCompositeId[0:repoEnd]
		idAndVersion = pluginCompositeId[repoEnd+1:]
	} else {
		idAndVersion = pluginCompositeId
	}

	parts := strings.Split(idAndVersion, ":")
	if len(parts) != 2 {
		return nil, errors.New("Tool Id should contain the plugin Id and the version separated by ':', but was: " + pluginCompositeId)
	}

	id := parts[0]
	version := parts[1]

	workDir, err := ioutil.TempDir("", "che-plugin-registry")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir) // clean up

	pluginMetaPath := filepath.Join(workDir, "meta.yaml")

	err = utils.New().Download(repo+"/"+id+"/"+version+"/meta.yaml", pluginMetaPath)
	if err != nil {
		return nil, err
	}

	f, err := ioutil.ReadFile(pluginMetaPath)
	if err != nil {
		return nil, err
	}

	pluginMeta := &model.PluginMeta{}
	if err := yaml.Unmarshal(f, pluginMeta); err != nil {
		return nil, err
	}

	return pluginMeta, nil
}
