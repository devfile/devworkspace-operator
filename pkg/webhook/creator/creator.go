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
package creator

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logf.Log.WithName("webhook.creator")

func Add(mgr manager.Manager) error {
	kubeCfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return err
	}

	log.Info("Configuring creator mutating webhook")
	mutateWebhook, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(mutateWebhookCfgName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		_, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(buildMutateWebhookCfg())
		if err != nil {
			return err
		}
		log.Info("Created creator mutating webhook configuration")
	} else {
		mutateWebhookCfg := buildMutateWebhookCfg()
		mutateWebhookCfg.ObjectMeta.ResourceVersion = mutateWebhook.ObjectMeta.ResourceVersion
		_, err := client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Update(mutateWebhookCfg)
		if err != nil {
			return err
		}
		log.Info("Updated creator mutating webhook configuration")
	}
	mgr.GetWebhookServer().Register(mutateWebhookPath, &webhook.Admission{Handler: &WorkspaceAnnotator{}})
	return nil
}
