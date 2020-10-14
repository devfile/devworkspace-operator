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

package workspace

import (
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/webhook/server"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CertManagerAnnotationName = "cert-manager.io"
	CertManagerInjectKey      = "cert-manager.io/inject-ca-from"
)

func GetWebhookAnnotations(client crclient.Client, namespace string) (map[string]string, error) {
	webhookAnnotations := make(map[string]string)

	certManagerSecret, err := isCertManagerSecret(client, namespace)
	if err != nil {
		log.Error(err, "Failed when attempting to check if the secret is annotated with cert manager")
		return webhookAnnotations, err
	}

	if certManagerSecret {
		webhookAnnotations[CertManagerInjectKey] = fmt.Sprintf("%s/%s", namespace, server.WebhookServerCertManagerCertificateName)
	}

	return webhookAnnotations, nil
}

func hasCertManagerAnnotation(annotations map[string]string) bool {
	for key, _ := range annotations {
		if strings.HasPrefix(key, CertManagerAnnotationName) {
			return true
		}
	}
	return false
}
