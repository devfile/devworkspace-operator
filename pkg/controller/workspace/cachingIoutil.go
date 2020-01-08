//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"sync"
	"github.com/eclipse/che-plugin-broker/utils"
	commonBroker "github.com/eclipse/che-plugin-broker/common"
	"io/ioutil"
	"os"
	"io"
	"path/filepath"

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/log"
)

type cache struct {
	tempDir string
	filenamesPerUrl map[string]string
	random commonBroker.Random
	mux sync.Mutex
}

var downloadCache *cache

func SetupDownloadCache() error {
	downloadTempDir, err := ioutil.TempDir("", "che-plugin-broker-httpcache")
	if err != nil {
		 return err
	}

	downloadCache = &cache {
		tempDir: downloadTempDir,
		random: commonBroker.NewRand(),
		filenamesPerUrl: map[string]string {},
	}
	return nil
}

func CleanDownloadCache() {
	if downloadCache != nil {
		os.RemoveAll(downloadCache.tempDir)
	}
}

type impl struct {
	delegate utils.IoUtil
}

// New creates an instance of IoUtil using the cached http client.
func NewCachingIoUtil() utils.IoUtil {
	return &impl{
		delegate: utils.New(),
	}
}

func (util *impl) Download(URL string, destPath string, useContentDisposition bool) (string, error) {
	downloadCache.mux.Lock()
	defer downloadCache.mux.Unlock()
	path, exists := downloadCache.filenamesPerUrl[URL]
	if !exists {
		for {
			cacheDirName := downloadCache.random.String(10)
			cacheDir := filepath.Join(downloadCache.tempDir, cacheDirName)
			os.MkdirAll(cacheDir, 755)
			destDir, destFilename := filepath.Split(filepath.Clean(destPath))
			path = filepath.Join(cacheDir, destFilename)
			if _, err := os.Stat(path); err != nil {
        if os.IsNotExist(err) {
						path, err := util.delegate.Download(URL, path, useContentDisposition)
						if err != nil {
							return "", err
						}
						_, destFilename = filepath.Split(path)
						destPath = filepath.Join(destDir, destFilename)
						downloadCache.filenamesPerUrl[URL] = path
            break
        } else {
					return "", err
				}
			}
		}
	} else {
		Log.Info("Retrieving URL '%s' from the local cache: %s", URL, path)
	}

	return destPath, util.CopyFile(path, destPath)
}

func (util *impl) MkDir(dir string) error {
	return util.delegate.MkDir(dir)
}

func (util *impl) Fetch(URL string) ([]byte, error) {
	return util.delegate.Fetch(URL)
}

func (util *impl) TempDir(baseDir string, prefix string) (dirPath string, err error) {
	return util.delegate.TempDir(baseDir, prefix)
}

func (util *impl) CopyResource(src string, dest string) error {
	return util.delegate.CopyResource(src, dest)
}

func (util *impl) CopyFile(src string, dest string) error {
	return util.delegate.CopyFile(src, dest)
}

func (util *impl) ResolveDestPath(filePath string, destDir string) string {
	return util.delegate.ResolveDestPath(filePath, destDir)
}

func (util *impl) ResolveDestPathFromURL(url string, destDir string) string {
	return util.delegate.ResolveDestPathFromURL(url, destDir)
}

func (util *impl) Unzip(arch string, dest string) error {
	return util.delegate.Unzip(arch, dest)
}

func (util *impl) Untar(tarPath string, dest string) error {
	return util.delegate.Untar(tarPath, dest)
}

func (util *impl) CreateFile(file string, tr io.Reader) error {
	return util.delegate.CreateFile(file, tr)
}

// GetFilesByGlob is a wrapper around filepath.Glob() to allow mocking in tests
func (util *impl) GetFilesByGlob(glob string) ([]string, error) {
	return util.delegate.GetFilesByGlob(glob)
}

// DeleteFiles is a wrapper around os.RemoveAll() to allow mocking in tests
func (util *impl) RemoveAll(path string) error {
	return util.delegate.RemoveAll(path)
}
