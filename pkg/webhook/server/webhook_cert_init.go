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
package server

import (
	"context"
	"errors"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	"github.com/che-incubator/che-workspace-operator/internal/controller"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InitWebhookServer setups TLS for the webhook server
func InitWebhookServer(ctx context.Context) error {
	crclient, err := controller.CreateClient()
	if err != nil {
		return err
	}

	ns, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	err = syncService(ctx, crclient, ns)
	if err != nil {
		return err
	}

	err = syncConfigMap(ctx, crclient, ns)
	if err != nil {
		return err
	}

	err = updateDeployment(ctx, crclient, ns)
	if err != nil {
		return err
	}

	return errors.New("TLS is setup. Controller needs to restart to apply changes")
}

func syncService(ctx context.Context, crclient client.Client, namespace string) error {
	secureService := getSecureServiceSpec(namespace)
	if err := crclient.Create(ctx, secureService); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg := &v1.Service{}
		err := crclient.Get(ctx, types.NamespacedName{
			Name:      secureService.Name,
			Namespace: secureService.Namespace,
		}, existingCfg)

		err = crclient.Update(ctx, secureService)
		if err != nil {
			return err
		}
		log.Info("Updated secure service")
	} else {
		log.Info("Created secure service")
	}
	return nil
}

func syncConfigMap(ctx context.Context, crclient client.Client, namespace string) error {
	secureConfigMap := getSecureConfigMapSpec(namespace)
	if err := crclient.Create(ctx, secureConfigMap); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		existingCfg := &v1.Service{}
		err := crclient.Get(ctx, types.NamespacedName{
			Name:      secureConfigMap.Name,
			Namespace: secureConfigMap.Namespace,
		}, existingCfg)

		err = crclient.Update(ctx, secureConfigMap)
		if err != nil {
			return err
		}
		log.Info("Updated secure configmap")
	} else {
		log.Info("Created secure configmap")
	}
	return nil
}

// Update the deployment with the volumes needed for webhook server if they aren't already present
func updateDeployment(ctx context.Context, crclient client.Client, namespace string) error {
	deployment, err := controller.FindControllerDeployment(ctx, crclient)
	if err != nil {
		return err
	}

	isLabelsMissing := appendLabelIfMissing(&deployment.Spec.Template.Labels)
	isPortMissing := appendPortIfMissing(&deployment.Spec.Template.Spec.Containers[0].Ports)
	isVolumeMissing := appendVolumeIfMissing(&deployment.Spec.Template.Spec.Volumes, getCertVolume())
	isVMMissing := appendVolumeMountIfMissing(&deployment.Spec.Template.Spec.Containers[0].VolumeMounts, getCertVolumeMount())

	// Only bother updating if one of these are missing
	if isPortMissing || isLabelsMissing || isVolumeMissing || isVMMissing {
		if err = crclient.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

// appendVolumeMountIfMissing appends the volume mount if it is missing. Indicates if the volume mount is missing with the return value
func appendVolumeMountIfMissing(volumeMounts *[]v1.VolumeMount, volumeMount v1.VolumeMount) bool {
	for _, vm := range *volumeMounts {
		if vm.Name == volumeMount.Name {
			return false
		}
	}
	*volumeMounts = append(*volumeMounts, volumeMount)
	return true
}

// appendVolumeIfMissing appends the volume if it is missing. Indicates if the volume is missing with the return value
func appendVolumeIfMissing(volumes *[]v1.Volume, volume v1.Volume) bool {
	for _, v := range *volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	*volumes = append(*volumes, volume)
	return true
}

// appendLabelIfMissing appends a label to the deployment if it is missing. Indicates if the label is missing with the return value
func appendLabelIfMissing(labels *map[string]string) bool {
	value, ok := (*labels)["app"]
	if !ok || value != "che-workspace-controller" {
		(*labels)["app"] = "che-workspace-controller"
		return true
	}
	return false
}

// appendPortIfMissing appends a port to the che-workspace-controller container. Indicates if the port is missing with the return value
func appendPortIfMissing(ports *[]v1.ContainerPort) bool {
	if len(*ports) == 0 || (*ports)[0].Name != webhookServerName {
		*ports = append(*ports, getPort())
		return true
	}
	return false
}
