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
	"sync"
	"github.com/eclipse/che-plugin-broker/utils"
	commonBroker "github.com/eclipse/che-plugin-broker/common"
	"io/ioutil"
	"os"
	"io"
	"path/filepath"	
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

func (util *impl) Download(URL string, destPath string) error {
	downloadCache.mux.Lock()
	defer downloadCache.mux.Unlock()
	path, exists := downloadCache.filenamesPerUrl[URL]
	if !exists {
		for {
			fileName := downloadCache.random.String(10)
			path = filepath.Join(downloadCache.tempDir, fileName)
			if _, err := os.Stat(path); err != nil {
        if os.IsNotExist(err) {
						err = util.delegate.Download(URL, path)
						if err != nil {
							return err
						}
						downloadCache.filenamesPerUrl[URL] = path
            break
        } else {
					return err
				}
			}
		}
	} else {
		log.Info(join ("",
		"Retrieving URL '",
		URL,
		"' from the local cache"))
	}
	return util.CopyFile(path, destPath)
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
