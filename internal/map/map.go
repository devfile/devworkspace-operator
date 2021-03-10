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

package maputils

func Append(target map[string]string, key, value string) map[string]string {
	if target == nil {
		target = map[string]string{}
	}
	target[key] = value
	return target
}

// Equal compares string maps for equality, regardless of order. Note that it treats
// a nil map as equal to an empty (but not nil) map.
func Equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bval, ok := b[k]; !ok || bval != v {
			return false
		}
	}
	return true
}
