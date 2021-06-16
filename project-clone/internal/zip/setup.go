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

package zip

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

const (
	tmpDir = "/tmp/"
)

func SetupZipProject(project v1alpha2.Project) error {
	url := project.Zip.Location
	clonePath := internal.GetClonePath(&project)

	zipFilePath := path.Join(tmpDir, fmt.Sprintf("%s.zip", clonePath))
	log.Printf("Downloading project archive from %s", url)
	err := downloadZip(url, zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to download archive: %s", err)
	}

	projectPath := path.Join(internal.ProjectsRoot, clonePath)

	log.Printf("Extracting project archive to %s", zipFilePath)
	err = unzip(zipFilePath, projectPath)
	if err != nil {
		return fmt.Errorf("failed to extract project zip archive: %s", err)
	}

	return nil
}

func downloadZip(url, destPath string) error {
	client := http.DefaultClient
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer closeSafe(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request at %s returned status code %s", url, resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer closeSafe(out)

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return out.Sync()
}

// unzip extracts an archive to a destination path.
//
// Adapted from the Che plugin broker:
// https://github.com/eclipse/che-plugin-broker/blob/27e7c6953c92633cbe7e8ce746a16ca10d240ea2/utils/ioutil.go#L190
func unzip(archivePath string, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer closeSafe(r)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		extractPath := filepath.Join(destPath, f.Name)

		if f.FileInfo().IsDir() {
			return os.MkdirAll(extractPath, 0755)
		} else {
			if err := os.MkdirAll(filepath.Dir(extractPath), 0775); err != nil {
				return err
			}
			f, err := os.OpenFile(extractPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}

			return f.Sync()
		}
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func closeSafe(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Println(err)
	}
}
