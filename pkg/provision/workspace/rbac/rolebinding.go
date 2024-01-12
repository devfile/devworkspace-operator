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

package rbac

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/dwerrors"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"

	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func syncRolebindings(workspace *common.DevWorkspaceWithConfig, api sync.ClusterAPI) error {
	saName := common.ServiceAccountName(workspace)
	defaultRoleName := common.WorkspaceRoleName()
	defaultRolebindingName := common.WorkspaceRolebindingName()
	if err := addServiceAccountToRolebinding(saName, workspace.Namespace, defaultRoleName, defaultRolebindingName, api); err != nil {
		return err
	}
	if !workspace.Spec.Template.Attributes.Exists(constants.WorkspaceSCCAttribute) {
		return nil
	}
	sccName := workspace.Spec.Template.Attributes.GetString(constants.WorkspaceSCCAttribute, nil)
	sccRoleName := common.WorkspaceSCCRoleName(sccName)
	sccRolebindingName := common.WorkspaceSCCRolebindingName(sccName)
	if err := addServiceAccountToRolebinding(saName, workspace.Namespace, sccRoleName, sccRolebindingName, api); err != nil {
		return err
	}
	return nil
}

func deleteRolebinding(name, namespace string, api sync.ClusterAPI) error {
	rolebinding := &rbacv1.RoleBinding{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, rolebinding)
	switch {
	case err == nil:
		if err := api.Client.Delete(api.Ctx, rolebinding); err != nil {
			return &dwerrors.RetryError{Message: fmt.Sprintf("failed to delete rolebinding %s in namespace %s", name, namespace), Err: err}
		}
		return &dwerrors.RetryError{Message: fmt.Sprintf("deleted rolebinding %s in namespace %s", name, namespace)}
	case k8sErrors.IsNotFound(err):
		// Already deleted
		return nil
	default:
		return err
	}
}

func addServiceAccountToRolebinding(saName, namespace, roleName, rolebindingName string, api sync.ClusterAPI) error {
	rolebinding := &rbacv1.RoleBinding{}
	namespacedName := types.NamespacedName{
		Name:      rolebindingName,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, rolebinding)
	switch {
	case err == nil:
		// Got existing rolebinding, need to make sure SA is part of it
		break
	case k8sErrors.IsNotFound(err):
		// Rolebinding not created yet, initiailize default rolebinding and add SA to it
		rolebinding = generateDefaultRolebinding(rolebindingName, namespace, roleName)
	default:
		return err
	}
	if !rolebindingHasSubject(rolebinding, saName, namespace) {
		rolebinding.Subjects = append(rolebinding.Subjects, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      saName,
			Namespace: namespace,
		})
	}

	if _, err = sync.SyncObjectWithCluster(rolebinding, api); err != nil {
		return dwerrors.WrapSyncError(err)
	}
	return nil
}

func removeServiceAccountFromRolebinding(saName, namespace, roleBindingName string, api sync.ClusterAPI) error {
	rolebinding := &rbacv1.RoleBinding{}
	namespacedName := types.NamespacedName{
		Name:      roleBindingName,
		Namespace: namespace,
	}
	err := api.Client.Get(api.Ctx, namespacedName, rolebinding)
	switch {
	case err == nil:
		// Found rolebinding, ensure saName is not a subject
		break
	case k8sErrors.IsNotFound(err):
		// Rolebinding does not exist; nothing to do
		return nil
	default:
		return err
	}
	if !rolebindingHasSubject(rolebinding, saName, namespace) {
		return nil
	}
	var newSubjects []rbacv1.Subject
	for _, subject := range rolebinding.Subjects {
		if subject.Kind == rbacv1.ServiceAccountKind && subject.Name == saName && subject.Namespace == namespace {
			continue
		}
		newSubjects = append(newSubjects, subject)
	}
	rolebinding.Subjects = newSubjects
	if _, err := sync.SyncObjectWithCluster(rolebinding, api); err != nil {
		return dwerrors.WrapSyncError(err)
	}
	return nil
}

func generateDefaultRolebinding(name, namespace, roleName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    rbacLabels,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: roleName,
		},
		// Subjects added for each workspace ServiceAccount
		Subjects: []rbacv1.Subject{},
	}
}

func rolebindingHasSubject(rolebinding *rbacv1.RoleBinding, saName, namespace string) bool {
	for _, subject := range rolebinding.Subjects {
		if subject.Kind == rbacv1.ServiceAccountKind && subject.Name == saName && subject.Namespace == namespace {
			return true
		}
	}
	return false
}
