//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package timing

import (
	"fmt"
	"strconv"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/config"
)

// IsEnabled returns whether storing timing info is enabled for the operator
func IsEnabled() bool {
	return config.ControllerCfg.GetExperimentalFeaturesEnabled()
}

// SetTime applies a given event annotation to the devworkspace with the current
// timestamp. No-op if timing is disabled or the annotation is already set, meaning
// this function can be called without additional checks.
func SetTime(timingInfo map[string]string, event string) {
	if !IsEnabled() {
		return
	}
	if timingInfo == nil {
		timingInfo = map[string]string{}
	}
	if _, set := timingInfo[event]; set {
		return
	}
	timingInfo[event] = strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
}

// SummarizeStartup applies aggregate annotations based off event annotations set by
// SetTime(). No-op if timing is disabled or if not all event annotations are present
// on the devworkspace.
func SummarizeStartup(workspace *dw.DevWorkspace) {
	if !IsEnabled() {
		return
	}
	times, err := getTimestamps(workspace)
	if err != nil {
		return
	}
	totalTime := times.serversReady - times.workspaceStarted
	workspace.Annotations[workspaceTotalTime] = fmt.Sprintf("%d ms", totalTime)
	componentsTime := times.componentsReady - times.componentsCreated
	workspace.Annotations[workspaceComponentsTime] = fmt.Sprintf("%d ms", componentsTime)
	routingsTime := times.routingReady - times.routingCreated
	workspace.Annotations[workspaceRoutingsTime] = fmt.Sprintf("%d ms", routingsTime)
	deploymentTime := times.deploymentReady - times.deploymentCreated
	workspace.Annotations[workspaceDeploymentTime] = fmt.Sprintf("%d ms", deploymentTime)
	serversTime := times.serversReady - times.deploymentReady
	workspace.Annotations[workspaceServersTime] = fmt.Sprintf("%d ms", serversTime)
}

// ClearAnnotations removes all timing-related annotations from a DevWorkspace.
// It's necessary to call this before setting new times via SetTime(), as SetTime()
// does not overwrite existing annotations.
func ClearAnnotations(workspace *dw.DevWorkspace) {
	if !IsEnabled() {
		return
	}
	delete(workspace.Annotations, DevWorkspaceStarted)
	delete(workspace.Annotations, ComponentsCreated)
	delete(workspace.Annotations, ComponentsReady)
	delete(workspace.Annotations, RoutingCreated)
	delete(workspace.Annotations, RoutingReady)
	delete(workspace.Annotations, DeploymentCreated)
	delete(workspace.Annotations, DeploymentReady)
	delete(workspace.Annotations, DevWorkspaceReady)
	delete(workspace.Annotations, workspaceTotalTime)
	delete(workspace.Annotations, workspaceComponentsTime)
	delete(workspace.Annotations, workspaceRoutingsTime)
	delete(workspace.Annotations, workspaceDeploymentTime)
	delete(workspace.Annotations, workspaceServersTime)
}
