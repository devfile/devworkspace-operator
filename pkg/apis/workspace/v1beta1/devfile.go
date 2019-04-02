package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// This schema describes the structure of the devfile object
type DevFileSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Commands          []CommandSpec   `json:"commands,omitempty"` // Description of the predefined commands to be available in workspace
	Projects          []ProjectSpec   `json:"projects,omitempty"` // Description of the projects, containing names and sources locations
	Components        []ComponentSpec `json:"components"`         // Description of the workspace components, such as editor and plugins
}

type CommandSpec struct {
	Actions    []CommandActionSpec `json:"actions,omitempty"`    // List of the actions of given command. Now the only one command must be specified in list; but there are plans to implement supporting multiple actions commands.
	Attributes map[string]string   `json:"attributes,omitempty"` // Additional command attributes
	Name       string              `json:"name"`                 // Describes the name of the command. Should be unique per commands set.
}

type CommandActionSpec struct {
	Command   *string `json:"command,omitempty"` // The actual action command-line string
	Component string  `json:"component"`         // Describes component to which given action relates
	Type      string  `json:"type"`              // Describes action type
	Workdir   *string `json:"workdir,omitempty"` // Working directory where the command should be executed
}

type ProjectSpec struct {
	Name   string            `json:"name"`
	Source ProjectSourceSpec `json:"source"` // Describes the project's source - type and location
}

// Describes the project's source - type and location
type ProjectSourceSpec struct {
	Location string `json:"location"` // Project's source location address. Should be URL for git and github located projects, or; file:// for zip.
	Type     string `json:"type"`     // Project's source type.
}

type ComponentSpec struct {
	Endpoints        []Endpoint        `json:"endpoints,omitempty"`        // Describes dockerimage component endpoints
	Env              []Env             `json:"env,omitempty"`              // The environment variables list that should be set to docker container
	Id               *string           `json:"id,omitempty"`               // Describes the component FQN
	Image            *string           `json:"image,omitempty"`            // Specifies the docker image that should be used for component
	Reference        *string           `json:"reference,omitempty"`        // Describes location of Kubernetes list yaml file. Applicable only for 'kubernetes' and; 'openshift' type components
	ReferenceContent *string           `json:"referenceContent,omitempty"` // Inlined content of a file specified in field 'local'
	MemoryLimit      *string           `json:"memoryLimit,omitempty"`      // Describes memory limit for the component. You can express memory as a plain integer or as a; fixed-point integer using one of these suffixes: E, P, T, G, M, K. You can also use the; power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki
	MountSources     *bool             `json:"mountSources,omitempty"`     // Describes whether projects sources should be mount to the component. `CHE_PROJECTS_ROOT`; environment variable should contains a path where projects sources are mount
	Name             string            `json:"name"`                       // Describes the name of the component. Should be unique per component set.
	Selector         map[string]string `json:"selector,omitempty"`         // Describes the objects selector for the recipe type components. Allows to pick-up only selected; items from k8s/openshift list
	Type             DevfileName       `json:"type"`                       // Describes type of the component, e.g. whether it is an plugin or editor or other type
	Volumes          []Volume          `json:"volumes,omitempty"`          // Describes volumes which should be mount to component
	Command          *[]string         `json:"command,omitempty"`          // The command to run in the dockerimage component instead of the default one provided in the image. Defaults to null, meaning use whatever is defined in the image.
	Args             *[]string         `json:"args,omitempty"`             // The arguments to supply to the command running the dockerimage component. The arguments are supplied either to the default command provided in the image or to the overridden command. Defaults to null, meaning use whatever is defined in the image.
}

// Describes dockerimage component endpoint
type Endpoint struct {
	Attributes *EndpointAttributes `json:"attributes,omitempty"`
	Name       string              `json:"name"` // The endpoint name
	Port       int64               `json:"port"` // The endpoint port
}

type EndpointAttributes struct {
	AdditionalAttributes *map[string]string `json:",inline,omitempty"`
	Public               *bool              `json:"public,omitempty"`       // Identifies endpoint as workspace internally or externally accessible - default: true
	Secure               *bool              `json:"secure,omitempty"`       // Identifies server as secure or non-secure. Requests to secure servers will be authenticated and must contain machine token - default: false
	Discoverable         *bool              `json:"discoverable,omitempty"` // Identifies endpoint as accessible by its name. - default: false
	Protocol             *string            `json:"protocol,omitempty"`     // Defines protocol that should be used for communication with endpoint. Is used for endpoint URL evaluation"
	Path                 *string            `json:"path,omitempty"`         // Defines path that should be used for communication with endpoint. Is used for endpoint URL evaluation"
}

// Describes environment variable
type Env struct {
	Name  string `json:"name"`  // The environment variable name
	Value string `json:"value"` // The environment variable value
}

// Describe volume that should be mount to component
type Volume struct {
	ContainerPath string `json:"containerPath"`
	Name          string `json:"name"` // The volume name. If several components mount the same volume then they will reuse the volume; and will be able to access to the same files
}

type DevfileName string

const (
	CheEditor   DevfileName = "cheEditor"
	ChePlugin   DevfileName = "chePlugin"
	Dockerimage DevfileName = "dockerimage"
	Kubernetes  DevfileName = "kubernetes"
	Openshift   DevfileName = "openshift"
)
