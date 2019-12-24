package model

import (
	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec   ComponentInstanceSpec
	Status ComponentInstanceStatus
}

type ComponentInstanceSpec struct {
	ComponentClass string                           `json:"componentClass"`
	component      workspacev1alpha1.ComponentSpec  `json:"component"`
}

type MachineDescription struct {
	MachineAttributes               map[string]string  `json:"machineAttributes,omitempty"`
	Ports                           []int              `json:"ports,omitempty"`
}

type ComponentInstanceStatus struct {
	Machines                        map[string]MachineDescription  `json:"machines,omitempty"`
	ContributedRuntimeCommands      []CheWorkspaceCommand          `json:"contributedRuntimeCommands,omitempty"`
	WorkspacePodAdditions           *corev1.PodTemplateSpec        `json:"-"`
	ExternalObjects                 []runtime.Object               `json:"-"`
	PluginFQN                       *model.PluginFQN               `json:"-"`
	Endpoints                       []workspacev1alpha1.Endpoint   `json:"-"`
}
