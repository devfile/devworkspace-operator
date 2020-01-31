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

import (
	workspacev1alpha1 "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ComponentInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ComponentInstanceSpec
	Status            ComponentInstanceStatus
}

type ComponentInstanceSpec struct {
	ComponentClass string                          `json:"componentClass"`
	component      workspacev1alpha1.ComponentSpec `json:"component"`
}

type MachineDescription struct {
	MachineAttributes map[string]string `json:"machineAttributes,omitempty"`
	Ports             []int             `json:"ports,omitempty"`
}

type ComponentInstanceStatus struct {
	Machines              map[string]MachineDescription `json:"machines,omitempty"`
	WorkspacePodAdditions *corev1.PodTemplateSpec       `json:"-"`
	ExternalObjects       []runtime.Object              `json:"-"`
	PluginFQN             *model.PluginFQN              `json:"-"`
	Endpoints             []workspacev1alpha1.Endpoint  `json:"-"`
	//fields below are used to be propagated via Che REST API Emulator for workspace components
	ContributedRuntimeCommands []CheWorkspaceCommand `json:"contributedRuntimeCommands,omitempty"`
}
