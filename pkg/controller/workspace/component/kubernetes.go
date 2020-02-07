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

package component

import (
	"errors"
	"strconv"

	routeV1 "github.com/openshift/api/route/v1"
	templateV1 "github.com/openshift/api/template/v1"
	"k8s.io/client-go/kubernetes/scheme"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"

	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/log"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

func setupK8sLikeComponent(wkspProps WorkspaceProperties, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
	var k8sObjects []runtime.Object

	theScheme := runtime.NewScheme()
	scheme.AddToScheme(theScheme)
	if component.Type == workspaceApi.Openshift {
		templateV1.AddToScheme(theScheme)
		routeV1.AddToScheme(theScheme)
	}

	decode := serializer.NewCodecFactory(theScheme).UniversalDeserializer().Decode

	componentContent := ""
	if component.Reference != nil {

	} else if component.ReferenceContent != nil {
		componentContent = *component.ReferenceContent
	}

	obj, _, err := decode([]byte(componentContent), nil, nil)
	if err != nil {
		return nil, err
	}

	var objects []runtime.Object
	if list, isList := obj.(*corev1.List); isList {
		items := list.Items
		for _, item := range items {
			if item.Object != nil {
				objects = append(objects, item.Object)
			} else {
				decodedItem, _, err := decode(item.Raw, nil, nil)
				if err != nil {
					return nil, err
				}
				if decodedItem != nil {
					objects = append(objects, decodedItem)
				} else {
					Log.Info("Unknown object ignored in the `kubernetes` component: " + string(item.Raw))
				}
			}
		}
	} else {
		objects = append(objects, obj)
	}

	selector := labels.SelectorFromSet(component.Selector)
	for _, obj = range objects {
		if objMeta, isMeta := obj.(metav1.Object); isMeta {
			if selector.Matches(labels.Set(objMeta.GetLabels())) {
				objMeta.SetNamespace(wkspProps.Namespace)
				k8sObjects = append(k8sObjects, obj)
			}
		}
	}

	var componentName string

	// TODO: manage commands and args with pod name, container name, etc ...

	podCount := 0
	for index, obj := range k8sObjects {
		if objPod, isPod := obj.(*corev1.Pod); isPod {
			componentName = objPod.Spec.Containers[0].Name
			podCount++
			suffix := ""
			if podCount > 1 {
				suffix = "." + strconv.Itoa(podCount)
			}
			additionalDeploymentOriginalName := componentName + suffix
			var workspaceDeploymentName = wkspProps.WorkspaceId + "." + additionalDeploymentOriginalName
			var replicas int32 = 1
			fromIntOne := intstr.FromInt(1)

			podLabels := map[string]string{
				"deployment":         workspaceDeploymentName,
				WorkspaceIDLabel:     wkspProps.WorkspaceId,
				CheOriginalNameLabel: additionalDeploymentOriginalName,
				WorkspaceNameLabel:   wkspProps.WorkspaceName,
			}
			for labelName, labelValue := range objPod.Labels {
				if _, exists := podLabels[labelName]; exists {
					return nil, errors.New("Label reserved by Che: " + labelName)
				}
				podLabels[labelName] = labelValue
			}

			k8sObjects[index] = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workspaceDeploymentName,
					Namespace: wkspProps.Namespace,
					Labels: map[string]string{
						WorkspaceIDLabel: wkspProps.WorkspaceId,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"deployment":         workspaceDeploymentName,
							WorkspaceIDLabel:     wkspProps.WorkspaceId,
							CheOriginalNameLabel: additionalDeploymentOriginalName,
						},
					},
					Replicas: &replicas,
					Strategy: appsv1.DeploymentStrategy{
						Type: "RollingUpdate",
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge:       &fromIntOne,
							MaxUnavailable: &fromIntOne,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:        objPod.Name,
							Labels:      podLabels,
							Annotations: objPod.Annotations,
						},
						Spec: objPod.Spec,
					},
				},
			}
		} else if objMeta, isMeta := obj.(metav1.Object); isMeta {
			objMeta.GetLabels()[WorkspaceIDLabel] = wkspProps.WorkspaceId
		}
	}

	componentInstanceStatus := &ComponentInstanceStatus{
		Containers:                 map[string]ContainerDescription{},
		ExternalObjects:            []runtime.Object{},
		Endpoints:                  []workspaceApi.Endpoint{},
		ContributedRuntimeCommands: []CheWorkspaceCommand{},
	}

	return componentInstanceStatus, nil
}
