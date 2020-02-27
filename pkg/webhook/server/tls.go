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
package server

import (
	"context"
	"github.com/che-incubator/che-workspace-operator/internal/ownerref"
	tlsutil "github.com/operator-framework/operator-sdk/pkg/tls"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func generateTLSCerts(mgr manager.Manager, ctx context.Context) error {
	//TODO Refactor this func
	kubeCfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return err
	}
	certGenerator := tlsutil.NewSDKCertGenerator(client)

	crclient, err := createClient()
	if err != nil {
		return err
	}

	deployment, err := ownerref.FindControllerDeployment(ctx, crclient)
	if err != nil {
		return err
	}

	certConfig := &tlsutil.CertConfig{
		CertName:   "webhook-server",
		CommonName: "workspace-controller.che-workspace-controller.svc",
	}
	tlsSecret, CAConfigMap, CAKeySecret, err := certGenerator.GenerateCert(deployment, &v1.Service{}, certConfig)
	if err != nil {
		return err
	}

	ownRef, err := ownerref.FindControllerOwner(ctx, crclient)
	if err != nil {
		return err
	}
	//TODO Do not update if not needed
	tlsSecret.SetOwnerReferences([]metav1.OwnerReference{*ownRef})
	if err = crclient.Update(ctx, tlsSecret); err != nil {
		return err
	}

	CAConfigMap.SetOwnerReferences([]metav1.OwnerReference{*ownRef})
	if err = crclient.Update(ctx, CAConfigMap); err != nil {
		return err
	}

	CAKeySecret.SetOwnerReferences([]metav1.OwnerReference{*ownRef})
	if err = crclient.Update(ctx, CAKeySecret); err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Volumes = appendVolumeIfMissing(deployment.Spec.Template.Spec.Volumes,
		*&v1.Volume{
			Name: "ca-cert",
			VolumeSource: *&v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: CAConfigMap.Name,
					},
				},
			},
		})
	deployment.Spec.Template.Spec.Volumes = appendVolumeIfMissing(deployment.Spec.Template.Spec.Volumes,
		*&v1.Volume{
			Name: "tls-cert",
			VolumeSource: *&v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: tlsSecret.Name,
				},
			},
		})

	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = appendVolumeMountIfMissing(deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		*&v1.VolumeMount{
			Name:      "tls-cert",
			MountPath: webhookServerCertDir,
			ReadOnly:  true,
		})
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = appendVolumeMountIfMissing(deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		*&v1.VolumeMount{
			Name:      "ca-cert",
			MountPath: webhookCADir,
			ReadOnly:  true,
		})

	if err = crclient.Update(ctx, deployment); err != nil {
		return err
	}
	return nil
}

func appendVolumeMountIfMissing(volumeMounts []v1.VolumeMount, volumeMount v1.VolumeMount) []v1.VolumeMount {
	for _, vm := range volumeMounts {
		if vm.Name == volumeMount.Name {
			return volumeMounts
		}
	}
	return append(volumeMounts, volumeMount)
}

func appendVolumeIfMissing(volumes []v1.Volume, volume v1.Volume) []v1.Volume {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return volumes
		}
	}
	return append(volumes, volume)
}

func createClient() (crclient.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	client, err := crclient.New(cfg, crclient.Options{})
	if err != nil {
		return nil, err
	}

	return client, nil
}
