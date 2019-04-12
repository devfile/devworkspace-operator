package workspace

import (
	"strconv"
	"errors"

	routeV1 "github.com/openshift/api/route/v1"
	templateV1 "github.com/openshift/api/template/v1"
	"k8s.io/client-go/kubernetes/scheme"

	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func setupK8sLikeComponent(wkspProps workspaceProperties, component *workspaceApi.ComponentSpec, podSpec *corev1.PodSpec) ([]runtime.Object, error) {
	var k8sObjects []runtime.Object

	theScheme := runtime.NewScheme()
	scheme.AddToScheme(theScheme)
	if component.Type == "openshift" {
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

	objects := []runtime.Object{}
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
					log.Info("Unknown object ignored in the `kubernetes` component: " + string(item.Raw))
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
				objMeta.SetNamespace(wkspProps.namespace)
				k8sObjects = append(k8sObjects, obj)
			}
		}
	}

	// TODO: manage commands and args with pod name, container name, etc ...

	podCound := 0
	for index, obj := range k8sObjects {
		if objPod, isPod := obj.(*corev1.Pod); isPod {
			podCound ++
			suffix := ""
			if podCound > 1 {
				suffix = "." + strconv.Itoa(podCound)
			}
			additionalDeploymentOriginalName := component.Name + suffix
			var workspaceDeploymentName = wkspProps.workspaceId + "." + additionalDeploymentOriginalName
			var replicas int32 = 1
			fromIntOne := intstr.FromInt(1)

			podLabels := map[string]string{
				"deployment":       workspaceDeploymentName,
				"che.workspace_id": wkspProps.workspaceId,
				"che.original_name": additionalDeploymentOriginalName,
				"che.workspace_name":  wkspProps.workspaceName,
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
					Namespace: wkspProps.namespace,
					Labels: map[string]string{
						"che.workspace_id": wkspProps.workspaceId,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"deployment":       workspaceDeploymentName,
							"che.workspace_id": wkspProps.workspaceId,
							"che.original_name": additionalDeploymentOriginalName,
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
		}
	}

	return k8sObjects, nil
}
