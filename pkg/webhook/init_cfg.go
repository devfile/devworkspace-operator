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

package webhook

import (
	"context"
	"fmt"

	admv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/webhook/workspace"
)

// WebhookCfgsInit initializes the webhook that denies everything until webhook server is started successfully
func WebhookCfgsInit(client crclient.Client, ctx context.Context, namespace string) error {
	configuration := workspace.BuildMutateWebhookCfg(namespace)

	err := client.Create(ctx, configuration, &crclient.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info(fmt.Sprintf("Mutating webhooks configuration %s already exists", configuration.Name))
			return checkExistingConfigForConflict(client, ctx, namespace)
		} else {
			return err
		}
	}
	log.Info(fmt.Sprintf("Created webhooks configuration %s", configuration.Name))
	return nil
}

func checkExistingConfigForConflict(client crclient.Client, ctx context.Context, serviceNamespace string) error {
	existingCfg := &admv1.MutatingWebhookConfiguration{}
	err := client.Get(ctx, types.NamespacedName{Name: workspace.MutateWebhookCfgName}, existingCfg)
	if err != nil {
		return err
	}
	for _, webhook := range existingCfg.Webhooks {
		if webhook.ClientConfig.Service.Namespace != serviceNamespace {
			return fmt.Errorf("conflicting webhook definition found on cluster, webhook %s clientConfig points at namespace %s", webhook.Name, webhook.ClientConfig.Service.Namespace)
		}
	}
	return nil
}
