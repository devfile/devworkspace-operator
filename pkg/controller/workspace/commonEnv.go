package workspace

import (
	corev1 "k8s.io/api/core/v1"
)

func commonEnvironmentVariables(wkspProps workspaceProperties) []corev1.EnvVar {
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
			Value: defaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_INTERNAL",
			Value: defaultApiEndpoint,
		},
		corev1.EnvVar{
			Name:  "CHE_API_EXTERNAL",
			Value: wkspProps.cheApiExternal,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAME",
			Value: wkspProps.workspaceName,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_ID",
			Value: wkspProps.workspaceId,
		},
		corev1.EnvVar{
			Name:  "CHE_AUTH_ENABLED",
			Value: authEnabled,
		},
		corev1.EnvVar{
			Name:  "CHE_WORKSPACE_NAMESPACE",
			Value: wkspProps.namespace,
		},
	}
}
