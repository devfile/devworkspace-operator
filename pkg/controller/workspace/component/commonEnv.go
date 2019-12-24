package component

import (
	corev1 "k8s.io/api/core/v1"

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func commonEnvironmentVariables(wkspProps WorkspaceProperties) []corev1.EnvVar {
	return []corev1.EnvVar{
		corev1.EnvVar{
			Name: "CHE_MACHINE_TOKEN",
		},
		corev1.EnvVar{
			Name:  "CHE_PROJECTS_ROOT",
			Value: "/projects",
		},
		corev1.EnvVar{
			Name:  "CHE_API",
			Value: wkspProps.CheApiExternal,
		},
		corev1.EnvVar{
			Name:  "CHE_API_INTERNAL",
			Value: DefaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_EXTERNAL",
			Value: wkspProps.CheApiExternal,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAME",
			Value: wkspProps.WorkspaceName,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_ID",
			Value: wkspProps.WorkspaceId,
		},
		corev1.EnvVar{
			Name:  "CHE_AUTH_ENABLED",
			Value: AuthEnabled,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: wkspProps.Namespace,
		},
	}
}
