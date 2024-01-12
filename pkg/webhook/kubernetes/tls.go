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

package webhook_k8s

import (
	"context"

	"github.com/devfile/devworkspace-operator/pkg/webhook/service"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("webhook-k8s")

// SetupSecureService handles TLS secrets required for deployment on Kubernetes.
func SetupSecureService(client crclient.Client, ctx context.Context, namespace string) error {
	err := service.CreateOrUpdateSecureService(client, ctx, namespace, map[string]string{})
	if err != nil {
		log.Info("Failed creating the secure service")
		return err
	}

	return nil
}
