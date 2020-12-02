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

package adaptor

// DownloadMetasError represents an error that occurs while downloading plugin meta.yamls
// This error wraps the underlying error that caused the failure.
type DownloadMetasError struct {
	Plugin string
	Err    error
}

var _ error = (*DownloadMetasError)(nil)

func (e *DownloadMetasError) Error() string {
	return "Failed to download plugin meta.yaml for " + e.Plugin
}
func (e *DownloadMetasError) Unwrap() error { return e.Err }
