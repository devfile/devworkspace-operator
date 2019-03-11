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
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func download(URL string, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func copyFile(src string, dest string) error {
	to, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer to.Close()

	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	_, err = io.Copy(to, from)
	return err
}

func resolveDestPath(filePath string, destDir string) string {
	destName := filepath.Base(filePath)
	destPath := filepath.Join(destDir, destName)
	return destPath
}

func resolveDestPathFromURL(url string, destDir string) string {
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	destPath := filepath.Join(destDir, fileName)
	return destPath
}

func untar(tarPath string, dest string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	gzr, err := gzip.NewReader(bufio.NewReader(file))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// no more files, unpacking is finished
		case err == io.EOF:
			return nil

		case err != nil:
			return err

			// skip empty header
		case header == nil:
			continue
		}

		tarEntry := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(tarEntry, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := createContainingDir(tarEntry); err != nil {
				return err
			}
			if err := createFile(tarEntry, tr); err != nil {
				return err
			}
		default:
			return errors.New("Unpacking archive problem occurred. Archive content has unexpected data")
		}
	}
}

func createContainingDir(filePath string) error {
	dirPath := filepath.Dir(filePath)
	return os.MkdirAll(dirPath, 0755)
}

func createFile(file string, tr io.Reader) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, tr); err != nil {
		return err
	}
	return f.Sync()
}

func clearDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			return err
		}
	}
	return nil
}
