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

package constants

const (
	// DevWorkspaceNamespace contains env var name which value is the current namespace where DevWorkspace CR
	// and related objects live
	DevWorkspaceNamespace = "DEVWORKSPACE_NAMESPACE"

	// DevWorkspaceId contains env var name which which value is the .status.devworkspaceId of the related
	// DevWorkspace CR. It can be used to list related objects with WorkspaceIDLabel selector
	DevWorkspaceId = "DEVWORKSPACE_ID"

	// DevWorkspaceName contains env var name which value is name of the related DevWorkspace CR.
	// It can be used to list related objects with WorkspaceNameLabel selector
	DevWorkspaceName = "DEVWORKSPACE_NAME"

	// DevWorkspaceCreator contains env var name which value is the uid of the identity who created the related devworkspace
	DevWorkspaceCreator = "DEVWORKSPACE_CREATOR"

	// DevWorkspaceIdleTimeout contains env var name which value is the suggested idle timeout
	DevWorkspaceIdleTimeout = "DEVWORKSPACE_IDLE_TIMEOUT"

	// DevWorkspaceComponentName contains env var name which indicates from which devfile container component
	// the container is created from. Note the flattened devfile is used to evaluate it.
	DevWorkspaceComponentName = "DEVWORKSPACE_COMPONENT_NAME"
)
