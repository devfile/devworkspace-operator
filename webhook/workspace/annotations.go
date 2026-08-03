//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

package workspace

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func getWebhookAnnotations(namespace string) map[string]string {
	annotations := map[string]string{}
	if infrastructure.CertManagerDetected() {
		annotations["cert-manager.io/inject-ca-from"] = fmt.Sprintf("%s/devworkspace-controller-serving-cert", namespace)
	} else if infrastructure.IsOpenShift() {
		annotations["service.beta.openshift.io/inject-cabundle"] = "true"
	}
	return annotations
}
