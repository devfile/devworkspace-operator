package provision

import (
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncRBAC(workspace *v1alpha1.Workspace, client client.Client, reqLogger logr.Logger) (err error) {
	rbac := generateRBAC(workspace.Namespace)

	if err := SyncObjects(rbac, client, reqLogger); err != nil {
		return err
	}
	return nil
}

func generateRBAC(namespace string) []runtime.Object {
	// TODO: The rolebindings here are created namespace-wide; find a way to limit this, given that each workspace
	return []runtime.Object{
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exec",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					Resources: []string{"pods/exec"},
					APIGroups: []string{""},
					Verbs:     []string{"create"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ServiceAccount + "-view",
				Namespace: namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "view",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "Group",
					Name: "system:serviceaccounts:" + namespace,
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.ServiceAccount + "-exec",
				Namespace: namespace,
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "exec",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "Group",
					Name: "system:serviceaccounts:" + namespace,
				},
			},
		},
	}
}
