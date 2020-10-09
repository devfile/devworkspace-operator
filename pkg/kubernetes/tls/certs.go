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

package tls

import (
	"context"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/config"

	"github.com/devfile/devworkspace-operator/internal/cluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type GenCertParams struct {
	RequesterName string
	Domain        string
	Namespace     string
	//Secret name that will contain tls.crt and tls.key which is going to be used by server
	TLSSecretName string
	//Secret name that will contain ca.crt that should be propagated to clients' trust store
	CASecretName string
}

//GenerateCerts generates TLS certificates according to the specified parameters
//Currently it's powered by Job which image lives in https://github.com/che-dockerfiles/che-tls-secret-creator
func GenerateCerts(client crclient.Client, ctx context.Context, params GenCertParams) error {
	// Remove CA certificate secret if any
	err := removeCACertificate(client, params.CASecretName, params.Namespace)
	if err != nil {
		return err
	}

	controllerSA, err := config.ControllerCfg.GetWorkspaceControllerSA()
	if err != nil {
		return err
	}

	jobEnvVars := map[string]string{
		"DOMAIN":                         params.Domain,
		"CHE_NAMESPACE":                  params.Namespace,
		"CHE_SERVER_TLS_SECRET_NAME":     params.TLSSecretName,
		"CHE_CA_CERTIFICATE_SECRET_NAME": params.CASecretName,
	}

	jobName := params.RequesterName + "-gen-cert"

	labels := make(map[string]string)
	labels["app.kubernetes.io/name"] = jobName
	labels["app.kubernetes.io/component"] = "cert-generator"

	job, err := getSpecJob(jobName, params.Namespace, labels, controllerSA, jobEnvVars)
	if err != nil {
		return err
	}

	err = cluster.SyncJobToCluster(client, ctx, job)
	if err != nil {
		return err
	}

	// Wait a maximum of 300 seconds for the job to be completed
	err = cluster.WaitForJobCompletion(client, jobName, params.Namespace, 300*time.Second)
	if err != nil {
		return err
	}

	// Clean up everything related to the job now that it should be finished
	err = cluster.CleanJob(client, jobName, params.Namespace)
	if err != nil {
		return err
	}

	return nil
}

// removeCACertificate removes a CA Cert. Used to clear out the old CACert when we are creating a new one
func removeCACertificate(client crclient.Client, certName, namespace string) error {
	caSelfSignedCertificateSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: certName}, caSelfSignedCertificateSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// We don't have anything to remove in this case since its already not found
			return nil

		} else {
			log.Error(err, "Error getting self-signed certificate secret "+certName)
			return err
		}
	}

	// Remove CA cert because TLS secret is missing (they should be generated together).
	if err = client.Delete(context.TODO(), caSelfSignedCertificateSecret); err != nil {
		log.Error(err, "Error deleting self-signed certificate secret "+certName)
		return err
	}

	return nil
}
