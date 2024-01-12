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

// Package storage contains library functions for provisioning volumes and volumeMounts in containers according to the
// volume components in a devfile. These functions also handle mounting project sources to containers that require it.
//
// TODO:
//   - Figure out how to handle 'size' parameter on volumes, given that we can't meaningfully use it for
//     common PVC-type storage
//   - Devfile API spec is unclear on how mountSources should be handled -- mountPath is assumed to be /projects
//     and volume name is assumed to be "projects"
//     see issues:
//   - https://github.com/devfile/api/issues/290
//   - https://github.com/devfile/api/issues/291
package storage
