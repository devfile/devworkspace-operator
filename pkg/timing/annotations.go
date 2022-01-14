//
// Copyright (c) 2019-2022 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package timing

import (
	"strconv"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
)

const (
	// DevWorkspaceStarted denotes when a workspace was first started
	DevWorkspaceStarted = "controller.devfile.io/timing.started"
	// ComponentsCreated denotes when components were created for the workspace
	ComponentsCreated = "controller.devfile.io/timing.components.created"
	// ComponentsReady denotes when components were ready for the workspace
	ComponentsReady = "controller.devfile.io/timing.components.ready"
	// RoutingCreated denotes when the devworkspacerouting was created for the workspace
	RoutingCreated = "controller.devfile.io/timing.routing.created"
	// RoutingReady denotes when the devworkspacerouting was ready for the workspace
	RoutingReady = "controller.devfile.io/timing.routing.ready"
	// DeploymentCreated denotes when the deployment was created for the workspace
	DeploymentCreated = "controller.devfile.io/timing.deployment.created"
	// DeploymentReady denotes when the deployment was ready for the workspace
	DeploymentReady = "controller.devfile.io/timing.deployment.ready"
	// DevWorkspaceReady denotes when all health checks were completed and the workspace was ready
	DevWorkspaceReady = "controller.devfile.io/timing.ready"
)

const (
	workspaceTotalTime      = "controller.devfile.io/timing.duration"
	workspaceComponentsTime = "controller.devfile.io/timing.components.duration"
	workspaceRoutingsTime   = "controller.devfile.io/timing.routing.duration"
	workspaceDeploymentTime = "controller.devfile.io/timing.deployment.duration"
	workspaceServersTime    = "controller.devfile.io/timing.healthchecks.duration"
)

type workspaceTimes struct {
	workspaceStarted  int64
	componentsCreated int64
	componentsReady   int64
	routingCreated    int64
	routingReady      int64
	deploymentCreated int64
	deploymentReady   int64
	serversReady      int64
}

func getTimestamps(workspace *dw.DevWorkspace) (*workspaceTimes, error) {
	times := &workspaceTimes{}
	// Will return an error if the annotation is unset
	t, err := strconv.ParseInt(workspace.Annotations[DevWorkspaceStarted], 10, 0)
	if err != nil {
		return nil, err
	}
	times.workspaceStarted = t
	t, err = strconv.ParseInt(workspace.Annotations[ComponentsCreated], 10, 0)
	if err != nil {
		return nil, err
	}
	times.componentsCreated = t
	t, err = strconv.ParseInt(workspace.Annotations[ComponentsReady], 10, 0)
	if err != nil {
		return nil, err
	}
	times.componentsReady = t
	t, err = strconv.ParseInt(workspace.Annotations[RoutingCreated], 10, 0)
	if err != nil {
		return nil, err
	}
	times.routingCreated = t
	t, err = strconv.ParseInt(workspace.Annotations[RoutingReady], 10, 0)
	if err != nil {
		return nil, err
	}
	times.routingReady = t
	t, err = strconv.ParseInt(workspace.Annotations[DeploymentCreated], 10, 0)
	if err != nil {
		return nil, err
	}
	times.deploymentCreated = t
	t, err = strconv.ParseInt(workspace.Annotations[DeploymentReady], 10, 0)
	if err != nil {
		return nil, err
	}
	times.deploymentReady = t
	t, err = strconv.ParseInt(workspace.Annotations[DevWorkspaceReady], 10, 0)
	if err != nil {
		return nil, err
	}
	times.serversReady = t
	return times, nil
}
