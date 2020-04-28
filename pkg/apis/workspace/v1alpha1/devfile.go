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

package v1alpha1

// This schema describes the structure of the devfile object
type DevfileSpec struct {
	// Devfile API version
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`

	// Devfile metadata
	DevfileMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Devfile attributes, e.g. persistVolumes
	DevfileAttributes `json:"attributes,omitempty" yaml:"attributes,omitempty"`

	// List of projects that should be imported into the workspace
	Projects []ProjectSpec `json:"projects,omitempty" yaml:"projects,omitempty"` // Description of the projects, containing names and sources locations

	// List of components (editor, plugins, containers, ...) that will provide the workspace features
	Components []ComponentSpec `json:"components" yaml:"components"` // Description of the workspace components, such as editor and plugins

	// List of workspace-wide commands that can be associated to a given component, in order to run in the related container
	Commands []CommandSpec `json:"commands,omitempty" yaml:"commands,omitempty"` // Description of the predefined commands to be available in workspace
}

type DevfileMeta struct {
	GenerateName string `json:"generateName,omitempty" yaml:"generateName,omitempty"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
}

type DevfileAttributes struct {
	PersistVolumes bool `json:"persistVolumes,omitempty" yaml:"persistVolumes,omitempty"`
	EditorFree     bool `json:"editorFree,omitempty" yaml:"editorFree,omitempty"`
}

type ProjectSpec struct {
	Name   string            `json:"name" yaml:"name"`
	Source ProjectSourceSpec `json:"source" yaml:"source"` // Describes the project's source - type and location
}

// Describes the project's source - type and location
type ProjectSourceSpec struct {
	Location string `json:"location" yaml:"location"` // Project's source location address. Should be URL for git and github located projects, or; file:// for zip.
	Type     string `json:"type" yaml:"type"`         // Project's source type.
}

type ComponentSpec struct {
	//provision fields for all components types

	Type  ComponentType `json:"type" yaml:"type"`                       // Describes type of the component, e.g. whether it is an plugin or editor or other type
	Alias string        `json:"alias,omitempty" yaml:"alias,omitempty"` // Describes the name of the component. Should be unique per component set.

	//provision fields for cheEditor&chePlugin types

	Id string `json:"id,omitempty" yaml:"id,omitempty"` // Describes the component FQN

	//provision fields for cheEditor&chePlugin&Kubernetes&OpenShift types

	Reference string `json:"reference,omitempty" yaml:"reference,omitempty"` // Describes location of Kubernetes list yaml file. Applicable only for 'kubernetes' and; 'openshift' type components

	//fields for dockerimage type

	Image        string     `json:"image,omitempty" yaml:"image,omitempty"`               // Specifies the docker image that should be used for component
	MemoryLimit  string     `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`   // Describes memory limit for the component. You can express memory as a plain integer or as a; fixed-point integer using one of these suffixes: E, P, T, G, M, K. You can also use the; power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki
	MountSources bool       `json:"mountSources,omitempty" yaml:"mountSources,omitempty"` // Describes whether projects sources should be mount to the component. `CHE_PROJECTS_ROOT`; environment variable should contains a path where projects sources are mount
	Endpoints    []Endpoint `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`       // Describes dockerimage component endpoints
	Env          []Env      `json:"env,omitempty" yaml:"env,omitempty"`                   // The environment variables list that should be set to docker container
	Volumes      []Volume   `json:"volumes,omitempty" yaml:"volumes,omitempty"`           // Describes volumes which should be mount to component
	Command      []string   `json:"command,omitempty" yaml:"command,omitempty"`           // The command to run in the dockerimage component instead of the default one provided in the image. Defaults to null, meaning use whatever is defined in the image.
	Args         []string   `json:"args,omitempty" yaml:"args,omitempty"`                 // The arguments to supply to the command running the dockerimage component. The arguments are supplied either to the default command provided in the image or to the overridden command. Defaults to null, meaning use whatever is defined in the image.

	//provision fields for kubernetes&openshift types

	ReferenceContent *string           `json:"referenceContent,omitempty" yaml:"referenceContent,omitempty"` // Inlined content of a file specified in field 'local'
	Selector         map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`                 // Describes the objects selector for the recipe type components. Allows to pick-up only selected; items from k8s/openshift list
}

// Describes dockerimage component endpoint
type Endpoint struct {
	Attributes map[EndpointAttribute]string `json:"attributes,omitempty" yaml:"attributes,omitempty"`
	Name       string                       `json:"name" yaml:"name"` // The endpoint name
	Port       int64                        `json:"port" yaml:"port"` // The endpoint port
}

type EndpointAttribute string

const (
	//endpoint attribute that is used to configure whether it should be available publicly or workspace only
	PUBLIC_ENDPOINT_ATTRIBUTE EndpointAttribute = "public"

	//endpoint attribute that is used to configure whether it should be covered with authentication or not
	SECURE_ENDPOINT_ATTRIBUTE EndpointAttribute = "secure"

	//endpoint attribute that indicates endpoint type
	//expected values: terminal, ide
	TYPE_ENDPOINT_ATTRIBUTE EndpointAttribute = "type"

	//endpoint attribute that indicates which protocol is used by backend application
	PROTOCOL_ENDPOINT_ATTRIBUTE EndpointAttribute = "protocol"

	//endpoint attribute that indicates which path should be used by default to access an application
	PATH_ENDPOINT_ATTRIBUTE EndpointAttribute = "path"

	DISCOVERABLE_ATTRIBUTE EndpointAttribute = "discoverable"
)

// Describes environment variable
type Env struct {
	Name  string `json:"name" yaml:"name"`   // The environment variable name
	Value string `json:"value" yaml:"value"` // The environment variable value
}

// Describe volume that should be mount to component
type Volume struct {
	ContainerPath string `json:"containerPath" yaml:"containerPath"`
	Name          string `json:"name" yaml:"name"` // The volume name. If several components mount the same volume then they will reuse the volume; and will be able to access to the same files
}

type ComponentType string

const (
	CheEditor   ComponentType = "cheEditor"
	ChePlugin   ComponentType = "chePlugin"
	Dockerimage ComponentType = "dockerimage"
	Kubernetes  ComponentType = "kubernetes"
	Openshift   ComponentType = "openshift"
)

type CommandSpec struct {
	Actions    []CommandActionSpec `json:"actions,omitempty" yaml:"actions,omitempty"`       // List of the actions of given command. Now the only one command must be specified in list; but there are plans to implement supporting multiple actions commands.
	Attributes map[string]string   `json:"attributes,omitempty" yaml:"attributes,omitempty"` // Additional command attributes
	Name       string              `json:"name" yaml:"name"`                                 // Describes the name of the command. Should be unique per commands set.
}

type CommandActionSpec struct {
	Command          string `json:"command,omitempty" yaml:"command,omitempty"`                   // The actual action command-line string
	Component        string `json:"component,omitempty" yaml:"component,omitempty"`               // Describes component to which given action relates
	Type             string `json:"type" yaml:"type"`                                             // Describes action type
	Workdir          string `json:"workdir,omitempty" yaml:"workdir,omitempty"`                   // Working directory where the command should be executed
	Reference        string `json:"reference,omitempty" yaml:"reference,omitempty"`               // Working directory where the command should be executed
	ReferenceContent string `json:"referenceContent,omitempty" yaml:"referenceContent,omitempty"` // Working directory where the command should be executed
}
