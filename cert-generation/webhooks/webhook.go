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

package webhooks

import (
	"log"

	"github.com/devfile/devworkspace-operator/pkg/webhook/workspace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// WebhookInit Initialize the webhook that denies everything until controller is started succesfully
func WebhookInit(clientset *kubernetes.Clientset, namespace string) error {
	configuration := workspace.BuildMutateWebhookCfg(namespace)

	// Create mutating webhook
	_, err := clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(configuration)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Printf("Mutating webhooks configuration %s already exists", configuration.Name)
			return nil
		} else {
			return err
		}
	}
	log.Printf("Created Mutating webhooks configuration %s", configuration.Name)
	return nil
}
