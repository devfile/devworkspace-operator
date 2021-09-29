//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
