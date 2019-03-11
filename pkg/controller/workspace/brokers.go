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
	"fmt"
	"github.com/eclipse/che-plugin-broker/model"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	stdLog "log"
	"os"
	"path/filepath"
)

func ProcessPlugin(meta *model.PluginMeta) (*model.ToolingConf, error) {
	printDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	url := meta.URL

	workDir, err := ioutil.TempDir("", "che-plugin-broker")
	if err != nil {
		return nil, err
	}

	archivePath := filepath.Join(workDir, "testArchive.tar.gz")
	pluginPath := filepath.Join(workDir, "testArchive")

	// Download an archive
	printDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = download(url, archivePath)
	if err != nil {
		return nil, err
	}

	// Untar it
	printDebug("Untarring archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = untar(archivePath, pluginPath)
	if err != nil {
		return nil, err
	}

	printDebug("Resolving Che plugins for '%s:%s'", meta.ID, meta.Version)
	var toolingConf *model.ToolingConf
	toolingConf, err = resolveToolingConfig(meta, pluginPath)
	if err != nil {
		return nil, err
	}

	printDebug("Copying dependencies for '%s:%s'", meta.ID, meta.Version)
	err = copyDependencies(pluginPath)
	if err != nil {
		return nil, err
	}

	return toolingConf, nil
}

func resolveToolingConfig(meta *model.PluginMeta, workDir string) (*model.ToolingConf, error) {
	toolingConfPath := filepath.Join(workDir, "che-plugin.yaml")
	f, err := ioutil.ReadFile(toolingConfPath)
	if err != nil {
		return nil, err
	}

	tooling := &model.ToolingConf{}
	if err := yaml.Unmarshal(f, tooling); err != nil {
		return nil, err
	}

	return tooling, nil
}

func copyDependencies(workDir string) error {
	depsConfPath := filepath.Join(workDir, "che-dependency.yaml")
	if _, err := os.Stat(depsConfPath); os.IsNotExist(err) {
		return nil
	}

	f, err := ioutil.ReadFile(depsConfPath)
	if err != nil {
		return err
	}

	deps := &model.CheDependencies{}
	if err := yaml.Unmarshal(f, deps); err != nil {
		return err
	}

	for _, dep := range deps.Plugins {
		switch {
		case dep.Location != "" && dep.URL != "":
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains both 'location' and 'url' fields while just one should be present", dep.ID, dep.Version)
			return errors.New(m)
		case dep.Location != "":
			fileDest := resolveDestPath(dep.Location, "/plugins")
			fileSrc := filepath.Join(workDir, dep.Location)
			printDebug("Copying file '%s' to '%s'", fileSrc, fileDest)
			if err = copyFile(fileSrc, fileDest); err != nil {
				return err
			}
		case dep.URL != "":
			fileDest := resolveDestPathFromURL(dep.URL, "/plugins")
			printDebug("Downloading file '%s' to '%s'", dep.URL, fileDest)
			if err = download(dep.URL, fileDest); err != nil {
				return err
			}
		default:
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains neither 'location' nor 'url' field", dep.ID, dep.Version)
			return errors.New(m)
		}
	}

	return nil
}

func printDebug(format string, v ...interface{}) {
	stdLog.Printf(format, v...)
}

func printInfo(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	stdLog.Print(message)
	log.Info(message)
}

func printFatal(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	stdLog.Fatal(message)
	log.Error(nil, format, v...)
}
