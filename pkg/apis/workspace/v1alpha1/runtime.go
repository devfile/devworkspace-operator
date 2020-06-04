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

//Runtime objects that is supposed to be serialized into additionalInfo of WorkspaceStatus
//and then propagated to Workspace Components via Che Rest API Emulator
type CheWorkspaceRuntime struct {
	ActiveEnv    string                         `json:"activeEnv,omitempty"`
	Commands     []CheWorkspaceCommand          `json:"commands,omitempty"`
	Machines     map[string]CheWorkspaceMachine `json:"machines,omitempty"`
	MachineToken string                         `json:"machineToken,omitempty"`
	Owner        string                         `json:"owner,omitempty"`
	Warnings     []CheWorkspaceWarning          `json:"warnings,omitempty"`
}

type CheWorkspaceWarning struct {
	Code    float64 `json:"code,omitempty"`
	Message string  `json:"message,omitempty"`
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
	Attributes map[string]string        `json:"attributes,omitempty"`
	Status     CheWorkspaceServerStatus `json:"status"`
	URL        string                   `json:"url,omitempty"`
}

type CheWorkspaceServerStatus string

const (
	StoppedServerStatus CheWorkspaceServerStatus = "STOPPED"
	RunningServerStatus CheWorkspaceServerStatus = "RUNNING"
	UnknownServerStatus CheWorkspaceServerStatus = "UNKNOWN"
)
