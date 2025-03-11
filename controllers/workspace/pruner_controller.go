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

package controllers

import (
	"context"
	"fmt"
	"strconv"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/go-logr/logr"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PrunerReconciler ensures that the pruning CronJob and ConfigMap are created.
type PrunerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

const (
	PrunerConfigMap              = "devworkspace-pruner"
	PrunerCronJobName            = "devworkspace-pruner"
	PrunerImage                  = "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest"
	PrunerRetainTime             = "2592000"
	PrunerClusterRoleBindingName = "devworkspace-pruner"
	PrunerClusterRoleName        = "devworkspace-pruner"
	PrunerSchedule               = "0 0 1 * *"
	PrunerScriptKey              = "devworkspace-pruner"
	PrunerScriptVolume           = "script"
	PrunerServiceAccountName     = "devworkspace-pruner"
)

// Reconcile ensures the CronJob and ConfigMap are in place.

// +kubebuilder:rbac:groups="",resources=devworkspaceoperatorconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *PrunerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pruner", req.NamespacedName)
	log.Info("Reconciling DevWorkspace pruner resources")

	config := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Suspend the CronJob if the feature is disabled
	if config.Config.Workspace == nil || config.Config.Workspace.CleanupCronJob == nil || config.Config.Workspace.CleanupCronJob.Enable == nil || !*config.Config.Workspace.CleanupCronJob.Enable {
		if err := r.suspendCronJob(ctx, req.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure the default ConfigMap is present
	if err := r.ensureConfigMap(ctx, req.Namespace); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure the user ConfigMap is present (if configured)
	if err := r.ensureCustomConfigMap(ctx, req.Namespace, config.Config.Workspace.CleanupCronJob); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure the ServiceAccount
	if err := r.ensureServiceAccount(ctx, req.Namespace); err != nil {
		return ctrl.Result{}, err
	}
	// Ensure the Role
	if err := r.ensureClusterRole(ctx, req.Namespace); err != nil {
		return ctrl.Result{}, err
	}
	// Ensure the RoleBinding
	if err := r.ensureClusterRoleBinding(ctx, req.Namespace); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile CronJob
	if err := r.ensureCronJob(ctx, req.Namespace, config.Config.Workspace.CleanupCronJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PrunerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.DevWorkspaceOperatorConfig{}).
		Complete(r)
}

func (r *PrunerReconciler) suspendCronJob(ctx context.Context, namespace string) error {
	var cronJob batchv1.CronJob
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: PrunerCronJobName}, &cronJob); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if cronJob.Spec.Suspend == nil || !*cronJob.Spec.Suspend {
		cronJob.Spec.Suspend = pointer.Bool(true)
		if err := r.Update(ctx, &cronJob); err != nil {
			return err
		}
	}

	return nil
}

func (r *PrunerReconciler) ensureServiceAccount(ctx context.Context, namespace string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: meta.ObjectMeta{
			Name:      PrunerServiceAccountName,
			Namespace: namespace,
			Labels:    resourceLabels(),
		},
	}
	if err := r.Create(ctx, sa); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *PrunerReconciler) ensureClusterRole(ctx context.Context, namespace string) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: meta.ObjectMeta{
			Name:      PrunerClusterRoleName,
			Namespace: namespace,
			Labels:    resourceLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"workspace.devfile.io"},
				Resources: []string{"devworkspaces"},
				Verbs:     []string{"get", "create", "delete", "list", "update", "patch", "watch"},
			},
		},
	}
	if err := r.Create(ctx, role); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	return nil
}

func (r *PrunerReconciler) ensureClusterRoleBinding(ctx context.Context, namespace string) error {
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Name:      PrunerClusterRoleBindingName,
			Namespace: namespace,
			Labels:    resourceLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     PrunerClusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PrunerServiceAccountName,
				Namespace: namespace,
			},
		},
	}
	if err := r.Create(ctx, roleBinding); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	return nil
}

func (r *PrunerReconciler) ensureConfigMap(ctx context.Context, namespace string) error {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: PrunerConfigMap}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, prunerDefaultConfigMap(namespace))
		}
		return err
	}
	return nil
}

func (r *PrunerReconciler) ensureCustomConfigMap(ctx context.Context, namespace string, config *controllerv1alpha1.CleanupCronJobConfig) error {
	if config == nil || config.CronJobScript == "" {
		return nil
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: config.CronJobScript}, cm); err != nil {
		return err
	}

	// check if PrunerScriptKey exists
	if _, ok := cm.Data[PrunerScriptKey]; !ok {
		return fmt.Errorf("ConfigMap %s does not contain key %s", config.CronJobScript, PrunerScriptKey)
	}

	return nil
}

func (r *PrunerReconciler) ensureCronJob(ctx context.Context, namespace string, config *controllerv1alpha1.CleanupCronJobConfig) error {
	suspend := true
	if config != nil && config.Enable != nil {
		suspend = !*config.Enable
	}
	image := PrunerImage
	if config != nil && config.Image != "" {
		image = config.Image
	}
	retainTime := PrunerRetainTime
	if config != nil && config.RetainTime != nil && *config.RetainTime != 0 {
		retainTime = strconv.FormatInt(int64(*config.RetainTime), 10)
	}
	dryRun := false
	if config != nil && config.DryRun != nil {
		dryRun = *config.DryRun
	}
	configMapName := PrunerConfigMap
	if config != nil && config.CronJobScript != "" {
		configMapName = config.CronJobScript
	}

	cronJob := &batchv1.CronJob{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: PrunerCronJobName}, cronJob); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, prunerCronJob(namespace, suspend, image, retainTime, configMapName))
		}
		return err
	}

	needUpdate := false
	// suspend
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend != suspend {
		cronJob.Spec.Suspend = &suspend
		needUpdate = true
	}
	// image
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image != image {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = image
		needUpdate = true
	}
	// envs
	containerEnvs := &cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
	envs := map[string]string{
		"RETAIN_TIME": retainTime,
		"DRY_RUN":     strconv.FormatBool(dryRun),
	}
	for name, val := range envs {
		found := false

		for i, env := range *containerEnvs {
			if env.Name == name {
				found = true
				if env.Value != val {
					(*containerEnvs)[i].Value = val
					needUpdate = true
					break
				}
			}
		}

		if !found {
			*containerEnvs = append(*containerEnvs, corev1.EnvVar{Name: name, Value: val})
			needUpdate = true
		}
	}
	// configMap
	if cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name != configMapName {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = configMapName
		needUpdate = true
	}
	if needUpdate {
		if err := r.Update(ctx, cronJob); err != nil {
			return err
		}
	}

	return nil
}

func prunerDefaultConfigMap(namespace string) *corev1.ConfigMap {
	labels := resourceLabels()
	labels[constants.DevWorkspaceWatchConfigMapLabel] = "true"

	return &corev1.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Name:      PrunerConfigMap,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			PrunerScriptKey: `#!/usr/bin/env bash
current_time=$(date +%s)
echo "Current time: ${current_time}"
echo "RETAIN_TIME: ${RETAIN_TIME}"
for namespace in $(oc get namespaces -l app.kubernetes.io/component=workspaces-namespace -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}')
do
  for workspace in $(oc get devworkspaces -n ${namespace} -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}')
  do
    last_start=$(date -d$(oc get devworkspace ${workspace} -n ${namespace} -o go-template='{{range .status.conditions}}{{if eq .type "Started"}}{{.lastTransitionTime}}{{end}}{{end}}') +%s)
    workspace_age=$(( ${current_time} - ${last_start} ))
    started=$(oc get devworkspace ${workspace} -n ${namespace} -o go-template='{{.spec.started}}')
    if [[ "$started" != "true" ]] && [[ ${workspace_age} -gt ${RETAIN_TIME} ]]
    then
      echo "Removing workspace: ${workspace} in ${namespace}"
      oc delete devworkspace ${workspace} -n ${namespace}
    fi
  done
done
`,
		},
	}
}

func prunerCronJob(namespace string, suspend bool, image, retainTime, configMapName string) *batchv1.CronJob {
	labels := resourceLabels()
	labels[constants.DevWorkspaceWatchCronJobLabel] = "true"

	return &batchv1.CronJob{
		ObjectMeta: meta.ObjectMeta{
			Name:      PrunerCronJobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   PrunerSchedule,
			SuccessfulJobsHistoryLimit: pointer.Int32(3),
			FailedJobsHistoryLimit:     pointer.Int32(3),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			Suspend:                    pointer.Bool(suspend),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyOnFailure,
							ServiceAccountName: PrunerServiceAccountName,
							Volumes: []corev1.Volume{
								{
									Name: PrunerScriptVolume,
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapName,
											},
											DefaultMode: pointer.Int32(0555),
											Items: []corev1.KeyToPath{
												{
													Key:  PrunerScriptKey,
													Path: "devworkspace-pruner.sh",
												},
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name:  "openshift-cli",
									Image: image,
									Env: []corev1.EnvVar{
										{
											Name:  "RETAIN_TIME",
											Value: retainTime,
										},
									},
									Command: []string{"/script/devworkspace-pruner.sh"},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("64Mi"),
										},
										Limits: corev1.ResourceList{
											"cpu":    resource.MustParse("100m"),
											"memory": resource.MustParse("64Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: "/script",
											Name:      PrunerScriptVolume,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceLabels() map[string]string {
	labels := constants.ControllerAppLabels()
	labels["app.kubernetes.io/name"] = "devworkspace-pruner"
	return labels
}
