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

	"github.com/che-incubator/che-workspace-operator/internal/controller"
	"github.com/che-incubator/che-workspace-operator/pkg/controller/ownerref"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// NAME of the pod that will run the cert setup
	NAME = "create-tls-cert"
)

// CreateCert a new TLS cert for the webhook
func CreateCert(ctx context.Context) error {
	log.Info("Configuring TLS Certs for webhook")

	clientset, err := getClientSet()
	if err != nil {
		return err
	}

	ns, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}

	listOptions := metav1.ListOptions{
		FieldSelector: "metadata.name=create-tls-cert",
	}

	// return early if the certs have already been generated
	if !hasCertGenerationOccured(clientset, listOptions, ns) {
		// wait for the create tls cert job to complete before updating the deployment
		err := createJob(clientset, listOptions, ns)
		if err != nil {
			log.Info(err.Error())
			return err
		}

		// Update the deployment with the volumes needed for webhook server if the secrets arent mounted
		updateDeploymentErr := updateDeployment(ctx)
		if updateDeploymentErr != nil {
			return updateDeploymentErr
		}
	}

	return nil
}

func getClientSet() (*kubernetes.Clientset, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// create a new job that will be created if certs are needed
func createJob(clientset *kubernetes.Clientset, listOptions metav1.ListOptions, ns string) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "create-tls-cert",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					ServiceAccountName: "che-workspace-controller",
					Containers: []v1.Container{
						{
							Name:  "cert",
							Image: "jpinkney/generate-certs:latest",
							Command: []string{
								"./generate_certificate.sh",
							},
						},
					},
					RestartPolicy: "OnFailure",
				},
			},
		},
	}
	job, err := clientset.BatchV1().Jobs(ns).Create(job)
	if err != nil {
		return err
	}
	return nil
}

// check if cert generation has happened by seeing if a job has finished
func hasCertGenerationOccured(clientset *kubernetes.Clientset, listOptions metav1.ListOptions, ns string) bool {
	jobList, err := clientset.BatchV1().Jobs(ns).List(listOptions)
	if err != nil {
		return false
	}
	if len(jobList.Items) > 0 {
		jobStatus := jobList.Items[0].Status
		return jobStatus.Failed > 0 || jobStatus.Active > 0 || jobStatus.Succeeded > 0
	}
	return false
}

// Update the deployment with the volumes needed for webhook server if they aren't already present
func updateDeployment(ctx context.Context) error {

	log.Info("Attempting to update the deployment with volumes if they are missing")
	crclient, err := controller.CreateClient()
	if err != nil {
		return err
	}

	deployment, err := ownerref.FindControllerDeployment(ctx, crclient)
	if err != nil {
		return err
	}

	isVolumeMissing := appendVolumeIfMissing(&deployment.Spec.Template.Spec.Volumes,
		*&v1.Volume{
			Name: "webhook-tls-certs",
			VolumeSource: *&v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "webhook-server-tls",
				},
			},
		})

	isVMMissing := appendVolumeMountIfMissing(&deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
		*&v1.VolumeMount{
			Name:      "webhook-tls-certs",
			MountPath: webhookServerCertDir,
			ReadOnly:  true,
		})

	// Only bother updating if the volume or volume mount are missing
	if isVolumeMissing || isVMMissing {
		log.Info("Attempting to update the deployment with correct volume and volume mounts for secrets")
		if err = crclient.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

// append the volume mount if it is missing. Indicates if the volume mount is missing
func appendVolumeMountIfMissing(volumeMounts *[]v1.VolumeMount, volumeMount v1.VolumeMount) bool {
	for _, vm := range *volumeMounts {
		if vm.Name == volumeMount.Name {
			return false
		}
	}
	*volumeMounts = append(*volumeMounts, volumeMount)
	return true
}

// append the volume if it is missing. Indicates if the volume is missing
func appendVolumeIfMissing(volumes *[]v1.Volume, volume v1.Volume) bool {
	for _, v := range *volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	*volumes = append(*volumes, volume)
	return true
}
