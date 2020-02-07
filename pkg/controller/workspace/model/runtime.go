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

package model

import "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"

//Runtime objects that is supposed to be serialized into additionalInfo of WorkspaceStatus
//and then propagated to Workspace Components via Che Rest API Emulator
type CheWorkspaceRuntime struct {
	ActiveEnv    *string                        `json:"activeEnv,omitempty"`
	Commands     []CheWorkspaceCommand          `json:"commands,omitempty"`
	Machines     map[string]CheWorkspaceMachine `json:"machines,omitempty"`
	MachineToken *string                        `json:"machineToken,omitempty"`
	Owner        *string                        `json:"owner,omitempty"`
	Warnings     []CheWorkspaceWarning          `json:"warnings,omitempty"`
}

type CheWorkspaceWarning struct {
	Code    *float64 `json:"code,omitempty"`
	Message *string  `json:"message,omitempty"`
}

type CheWorkspaceMachine struct {
	Attributes map[string]string             `json:"attributes,omitempty"`
	Servers    map[string]CheWorkspaceServer `json:"servers,omitempty"`
	Status     *CheWorkspaceMachineEventType `json:"status,omitempty"`
}

type CheWorkspaceMachineEventType string

const (
	FailedMachineEventType   CheWorkspaceMachineEventType = "FAILED"
	RunningMachineEventType  CheWorkspaceMachineEventType = "RUNNING"
	StartingMachineEventType CheWorkspaceMachineEventType = "STARTING"
	StoppedMachineEventType  CheWorkspaceMachineEventType = "STOPPED"
)

type CheWorkspaceServer struct {
	Attributes map[v1alpha1.EndpointAttribute]string `json:"attributes,omitempty"`
	Status     CheWorkspaceServerStatus              `json:"status"`
	URL        *string                               `json:"url,omitempty"`
}

type CheWorkspaceCommand struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	CommandLine string            `json:"commandLine"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type CheCoreRESTLink struct {
	Consumes    *string                           `json:"consumes,omitempty"`
	Href        *string                           `json:"href,omitempty"`
	Method      *string                           `json:"method,omitempty"`
	Parameters  []CheCoreRESTLinkParameter        `json:"parameters"`
	Produces    *string                           `json:"produces,omitempty"`
	Rel         *string                           `json:"rel,omitempty"`
	RequestBody *CheCoreRESTRequestBodyDescriptor `json:"requestBody,omitempty"`
}

type CheCoreRESTLinkParameter struct {
	DefaultValue *string                       `json:"defaultValue,omitempty"`
	Description  *string                       `json:"description,omitempty"`
	Name         *string                       `json:"name,omitempty"`
	Required     *bool                         `json:"required,omitempty"`
	Type         *CheCoreRESTLinkParameterType `json:"type,omitempty"`
	Valid        []string                      `json:"valid"`
}

type CheCoreRESTLinkParameterType string

const (
	Array   CheCoreRESTLinkParameterType = "Array"
	Boolean CheCoreRESTLinkParameterType = "Boolean"
	Number  CheCoreRESTLinkParameterType = "Number"
	Object  CheCoreRESTLinkParameterType = "Object"
	String  CheCoreRESTLinkParameterType = "String"
)

type CheCoreRESTRequestBodyDescriptor struct {
	Description *string `json:"description,omitempty"`
}

type CheWorkspaceServerStatus string

const (
	StoppedServerStatus CheWorkspaceServerStatus = "STOPPED"
	RunningServerStatus CheWorkspaceServerStatus = "RUNNING"
	UnknownServerStatus CheWorkspaceServerStatus = "UNKNOWN"
)
