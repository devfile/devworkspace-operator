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

	projectslib "github.com/devfile/devworkspace-operator/pkg/library/projects"
	"github.com/devfile/devworkspace-operator/project-clone/internal"
)

const (
	tmpDir = "/tmp/"
)

// SetupZipProject downloads and extracts a zip-type project to the corresponding clonePath.
func SetupZipProject(project v1alpha2.Project, httpClient *http.Client) error {
	if project.Zip == nil {
		return fmt.Errorf("project has no 'zip' source")
	}
	url := project.Zip.Location
	clonePath := projectslib.GetClonePath(&project)
	projectPath := path.Join(internal.ProjectsRoot, clonePath)
	if exists, err := internal.DirExists(projectPath); exists {
		// Assume project is already set up
		log.Printf("Project '%s' is already configured", project.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check path %s: %s", projectPath, err)
	}

	tmpProjectsPath := path.Join(internal.CloneTmpDir, clonePath)

	zipFilePath := path.Join(tmpDir, fmt.Sprintf("%s.zip", clonePath))
	log.Printf("Downloading project archive from %s", url)
	err := downloadZip(url, zipFilePath, httpClient)
	if err != nil {
		return fmt.Errorf("failed to download archive: %s", err)
	}

	log.Printf("Extracting project archive to %s", tmpProjectsPath)
	err = unzip(zipFilePath, tmpProjectsPath)
	if err != nil {
		return fmt.Errorf("failed to extract project zip archive: %s", err)
	}

	// Move unzipped project from tmp dir to final destination
	log.Printf("Moving extracted project archive to %s", projectPath)
	if err := os.Rename(tmpProjectsPath, projectPath); err != nil {
		return fmt.Errorf("failed to move unzipped project to PROJECTS_ROOT: %w", err)
	}

	err = dropTopLevelFolder(projectPath)
	if err != nil {
		return fmt.Errorf("failed to process extracted project archive: %s", err)
	}

	return nil
}

// downloadZip downloads file from `url` to `destPath`
//
// Adapted from the Che plugin broker:
// https://github.com/eclipse/che-plugin-broker/blob/27e7c6953c92633cbe7e8ce746a16ca10d240ea2/utils/ioutil.go#L67
func downloadZip(url, destPath string, httpClient *http.Client) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer closeSafe(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request at %s returned status code %d", url, resp.StatusCode)
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

// dropTopLevelFolder handles the case where a zip archive contains a single folder by moving all files in a
// single subdirectory into their parent directory. E.g., it converts
//
//	/projects/my-project/my-project-main/[files]
//
// to
//
//	/projects/my-project/[files]
//
// and removes directory /projects/my-project/my-project-master/
// If the specified path contains additional files or directories, no changes are made.
func dropTopLevelFolder(projectPath string) error {
	files, err := os.ReadDir(projectPath)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		// Do nothing if specified path doesn't contain only a single directory
		return nil
	}
	topLevelFolder := files[0]
	if !topLevelFolder.IsDir() {
		return nil
	}
	topLevelPath := path.Join(projectPath, topLevelFolder.Name())
	log.Printf("Moving files from %s to %s", topLevelPath, projectPath)
	topLevelContents, err := os.ReadDir(topLevelPath)
	if err != nil {
		return err
	}
	for _, file := range topLevelContents {
		oldPath := path.Join(topLevelPath, file.Name())
		newPath := path.Join(projectPath, file.Name())
		err := os.Rename(oldPath, newPath)
		if err != nil {
			return err
		}
	}
	err = os.Remove(topLevelPath)
	if err != nil {
		return err
	}

	return nil
}

// closeSafe is a wrapper on io.Closer.Close() that just prints an error if encountered.
//
// Adapted from the Che plugin broker:
// https://github.com/eclipse/che-plugin-broker/blob/27e7c6953c92633cbe7e8ce746a16ca10d240ea2/utils/ioutil.go#L326
func closeSafe(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Println(err)
	}
}
