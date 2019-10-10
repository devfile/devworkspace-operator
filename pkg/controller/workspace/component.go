package workspace

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
	componentClass string
	component      workspacev1alpha1.ComponentSpec
}

type MachineDescription struct {
	machineAttributes               map[string]string
	ports                           []int
}

type ComponentInstanceStatus struct {
	machines                        map[string]MachineDescription
	contributedRuntimeCommands      []CheWorkspaceCommand
	WorkspacePodAdditions           *corev1.PodTemplateSpec
	externalObjects                 []runtime.Object
	pluginFQN                       *model.PluginFQN
	endpoints                       []workspacev1alpha1.Endpoint
}
