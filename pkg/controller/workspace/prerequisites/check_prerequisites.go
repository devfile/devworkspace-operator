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

package prerequisites

import (
	"context"
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckPrerequisites(workspace *v1alpha1.Workspace, client client.Client, reqLogger logr.Logger) error {
	prereqs, err := generatePrerequisites(workspace.Namespace)
	if err != nil {
		return err
	}

	for _, prereq := range prereqs {
		prereqAsMetaObject, isMeta := prereq.(metav1.Object)
		if !isMeta {
			return errors.NewBadRequest("Converted objects are not valid K8s objects")
		}

		reqLogger.V(1).Info("Managing K8s Pre-requisite", "kind", reflect.TypeOf(prereq).Elem().String(), "name", prereqAsMetaObject.GetName())

		found := reflect.New(reflect.TypeOf(prereq).Elem()).Interface().(runtime.Object)
		err = client.Get(context.TODO(), types.NamespacedName{Name: prereqAsMetaObject.GetName(), Namespace: prereqAsMetaObject.GetNamespace()}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("    => Creating "+reflect.TypeOf(prereqAsMetaObject).Elem().String(), "namespace", prereqAsMetaObject.GetNamespace(), "name", prereqAsMetaObject.GetName())
			err = client.Create(context.TODO(), prereq)
			if err != nil {
				return err
			}
			continue
		} else if err != nil {
			return err
		} else {
			if _, isPVC := found.(*corev1.PersistentVolumeClaim); !isPVC {
				err = client.Update(context.TODO(), prereq)
				if err != nil {
					reqLogger.Error(err, "")
				}
			}
		}
	}
	return nil
}
