package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// This schema describes the structure of the devfile object
type DevFileSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Commands          []CommandSpec `json:"commands,omitempty"` // Description of the predefined commands to be available in workspace
	Projects          []ProjectSpec `json:"projects,omitempty"` // Description of the projects, containing names and sources locations
	Tools             []ToolSpec    `json:"tools"`              // Description of the workspace tools, such as editor and plugins
}

type CommandSpec struct {
	Actions    []CommandActionSpec `json:"actions,omitempty"`    // List of the actions of given command. Now the only one command must be specified in list; but there are plans to implement supporting multiple actions commands.
	Attributes map[string]string   `json:"attributes,omitempty"` // Additional command attributes
	Name       string              `json:"name"`                 // Describes the name of the command. Should be unique per commands set.
}

type CommandActionSpec struct {
	Command string  `json:"command"`           // The actual action command-line string
	Tool    string  `json:"tool"`              // Describes tool to which given action relates
	Type    string  `json:"type"`              // Describes action type
	Workdir *string `json:"workdir,omitempty"` // Working directory where the command should be executed
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

type ToolSpec struct {
	Endpoints    []Endpoint        `json:"endpoints,omitempty"`    // Describes dockerimage tool endpoints
	Env          []Env             `json:"env,omitempty"`          // The environment variables list that should be set to docker container
	Id           *string           `json:"id,omitempty"`           // Describes the tool FQN
	Image        *string           `json:"image,omitempty"`        // Specifies the docker image that should be used for tool
	Local        *string           `json:"local,omitempty"`        // Describes location of Kubernetes list yaml file. Applicable only for 'kubernetes' and; 'openshift' type tools
	LocalContent *string           `json:"localContent,omitempty"` // Inlined content of a file specified in field 'local'
	MemoryLimit  *string           `json:"memoryLimit,omitempty"`  // Describes memory limit for the tool. You can express memory as a plain integer or as a; fixed-point integer using one of these suffixes: E, P, T, G, M, K. You can also use the; power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki
	MountSources *bool             `json:"mountSources,omitempty"` // Describes whether projects sources should be mount to the tool. `CHE_PROJECTS_ROOT`; environment variable should contains a path where projects sources are mount
	Name         string            `json:"name"`                   // Describes the name of the tool. Should be unique per tool set.
	Selector     map[string]string `json:"selector,omitempty"`     // Describes the objects selector for the recipe type tools. Allows to pick-up only selected; items from k8s/openshift list
	Type         DevfileName       `json:"type"`                   // Describes type of the tool, e.g. whether it is an plugin or editor or other type
	Volumes      []Volume          `json:"volumes,omitempty"`      // Describes volumes which should be mount to tool
}

// Describes dockerimage tool endpoint
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

// Describe volume that should be mount to tool
type Volume struct {
	ContainerPath string `json:"containerPath"`
	Name          string `json:"name"` // The volume name. If several tools mount the same volume then they will reuse the volume; and will be able to access to the same files
}

type DevfileName string

const (
	CheEditor   DevfileName = "cheEditor"
	ChePlugin   DevfileName = "chePlugin"
	Dockerimage DevfileName = "dockerimage"
	Kubernetes  DevfileName = "kubernetes"
	Openshift   DevfileName = "openshift"
)
