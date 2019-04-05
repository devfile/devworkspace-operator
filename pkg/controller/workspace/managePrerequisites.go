package workspace

import (
	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func managePrerequisites(workspace *workspaceApi.Workspace) ([]runtime.Object, error) {
	pvcStorageQuantity, err := resource.ParseQuantity(pvcStorageSize)
	if err != nil {
		return nil, err
	}

	autoMountServiceAccount := true

	k8sObjects := []runtime.Object{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "claim-che-workspace",
				Namespace: workspace.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": pvcStorageQuantity,
					},
				},
				StorageClassName: workspaceConfig.getPVCStorageClassName(),
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccount,
				Namespace: workspace.Namespace,
			},
			AutomountServiceAccountToken: &autoMountServiceAccount,
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exec",
				Namespace: workspace.Namespace,
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1.PolicyRule{
					Resources: []string{"pods/exec"},
					APIGroups: []string{""},
					Verbs:     []string{"create"},
				},
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "view-workspaces",
				Namespace: workspace.Namespace,
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1.PolicyRule{
					Resources: []string{"workspaces"},
					APIGroups: []string{"workspace.che.eclipse.org"},
					Verbs:     []string{"get", "list"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccount + "-view",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "view",
			},
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccount + "-exec",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "exec",
			},
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccount + "-view-workspaces",
				Namespace: workspace.Namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "view-workspaces",
			},
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount,
					Namespace: workspace.Namespace,
				},
			},
		},
	}
	return k8sObjects, nil
}
