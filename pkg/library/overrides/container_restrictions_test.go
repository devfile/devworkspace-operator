// Copyright (c) 2019-2026 Red Hat, Inc.
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

package overrides

import (
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
)

func TestRestrictContainerOverride(t *testing.T) {
	tests := []struct {
		Name string

		Override         corev1.Container
		RestrictedFields []string

		IsErrorExpected bool
		ErrField        string
	}{
		{
			Name:     "no restricted fields allows everything",
			Override: corev1.Container{},
		},
		{
			Name:            "name always restricted",
			Override:        corev1.Container{Name: "test"},
			IsErrorExpected: true,
			ErrField:        "name",
		},
		{
			Name:            "image always restricted",
			Override:        corev1.Container{Image: "test"},
			IsErrorExpected: true,
			ErrField:        "image",
		},
		{
			Name:            "command always restricted",
			Override:        corev1.Container{Command: []string{}},
			IsErrorExpected: true,
			ErrField:        "command",
		},
		{
			Name:            "args always restricted",
			Override:        corev1.Container{Args: []string{}},
			IsErrorExpected: true,
			ErrField:        "args",
		},
		{
			Name:            "ports always restricted",
			Override:        corev1.Container{Ports: []corev1.ContainerPort{{}}},
			IsErrorExpected: true,
			ErrField:        "ports",
		},
		{
			Name:            "env always restricted",
			Override:        corev1.Container{Env: []corev1.EnvVar{{}}},
			IsErrorExpected: true,
			ErrField:        "env",
		},
		// ----------- WorkingDir -----------
		{
			Name:             "workingDir empty value not restricted",
			Override:         corev1.Container{WorkingDir: ""},
			RestrictedFields: []string{"workingDir"},
			IsErrorExpected:  false,
		},
		{
			Name:             "workingDir restricted by specific value match",
			Override:         corev1.Container{WorkingDir: "/tmp"},
			RestrictedFields: []string{"workingDir=/tmp"},
			IsErrorExpected:  true,
			ErrField:         "workingDir=/tmp",
		},
		{
			Name:             "workingDir restricted by any value",
			Override:         corev1.Container{WorkingDir: "/workspace"},
			RestrictedFields: []string{"workingDir"},
			IsErrorExpected:  true,
			ErrField:         "workingDir",
		},
		{
			Name:             "workingDir allowed when restricted value does not match",
			Override:         corev1.Container{WorkingDir: "/tmp"},
			RestrictedFields: []string{"workingDir=/root"},
			IsErrorExpected:  false,
		},
		// ----------- RestartPolicy -----------
		{
			Name:             "restartPolicy nil value not restricted",
			Override:         corev1.Container{RestartPolicy: nil},
			RestrictedFields: []string{"restartPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "restartPolicy restricted by specific value match",
			Override:         corev1.Container{RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways)},
			RestrictedFields: []string{"restartPolicy=Always"},
			IsErrorExpected:  true,
			ErrField:         "restartPolicy=Always",
		},
		{
			Name:             "restartPolicy restricted by any value",
			Override:         corev1.Container{RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways)},
			RestrictedFields: []string{"restartPolicy"},
			IsErrorExpected:  true,
			ErrField:         "restartPolicy",
		},
		{
			Name:             "restartPolicy allowed when restricted value does not match",
			Override:         corev1.Container{RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways)},
			RestrictedFields: []string{"restartPolicy=Never"},
			IsErrorExpected:  false,
		},
		// ----------- TerminationMessagePath -----------
		{
			Name:             "terminationMessagePath empty value not restricted",
			Override:         corev1.Container{TerminationMessagePath: ""},
			RestrictedFields: []string{"terminationMessagePath"},
			IsErrorExpected:  false,
		},
		{
			Name:             "terminationMessagePath restricted by specific value match",
			Override:         corev1.Container{TerminationMessagePath: "/dev/termination-log"},
			RestrictedFields: []string{"terminationMessagePath=/dev/termination-log"},
			IsErrorExpected:  true,
			ErrField:         "terminationMessagePath=/dev/termination-log",
		},
		{
			Name:             "terminationMessagePath restricted by any value",
			Override:         corev1.Container{TerminationMessagePath: "/dev/termination-log"},
			RestrictedFields: []string{"terminationMessagePath"},
			IsErrorExpected:  true,
			ErrField:         "terminationMessagePath",
		},
		{
			Name:             "terminationMessagePath allowed when restricted value does not match",
			Override:         corev1.Container{TerminationMessagePath: "/dev/termination-log"},
			RestrictedFields: []string{"terminationMessagePath=/other/path"},
			IsErrorExpected:  false,
		},
		// ----------- TerminationMessagePolicy -----------
		{
			Name:             "terminationMessagePolicy empty value not restricted",
			Override:         corev1.Container{TerminationMessagePolicy: ""},
			RestrictedFields: []string{"terminationMessagePolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "terminationMessagePolicy restricted by specific value match",
			Override:         corev1.Container{TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError},
			RestrictedFields: []string{"terminationMessagePolicy=FallbackToLogsOnError"},
			IsErrorExpected:  true,
			ErrField:         "terminationMessagePolicy=FallbackToLogsOnError",
		},
		{
			Name:             "terminationMessagePolicy restricted by any value",
			Override:         corev1.Container{TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError},
			RestrictedFields: []string{"terminationMessagePolicy"},
			IsErrorExpected:  true,
			ErrField:         "terminationMessagePolicy",
		},
		{
			Name:             "terminationMessagePolicy allowed when restricted value does not match",
			Override:         corev1.Container{TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError},
			RestrictedFields: []string{"terminationMessagePolicy=File"},
			IsErrorExpected:  false,
		},
		// ----------- ImagePullPolicy -----------
		{
			Name:             "imagePullPolicy empty value not restricted",
			Override:         corev1.Container{ImagePullPolicy: ""},
			RestrictedFields: []string{"imagePullPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "imagePullPolicy restricted by specific value match",
			Override:         corev1.Container{ImagePullPolicy: corev1.PullAlways},
			RestrictedFields: []string{"imagePullPolicy=Always"},
			IsErrorExpected:  true,
			ErrField:         "imagePullPolicy=Always",
		},
		{
			Name:             "imagePullPolicy restricted by any value",
			Override:         corev1.Container{ImagePullPolicy: corev1.PullAlways},
			RestrictedFields: []string{"imagePullPolicy"},
			IsErrorExpected:  true,
			ErrField:         "imagePullPolicy",
		},
		{
			Name:             "imagePullPolicy allowed when restricted value does not match",
			Override:         corev1.Container{ImagePullPolicy: corev1.PullAlways},
			RestrictedFields: []string{"imagePullPolicy=Never"},
			IsErrorExpected:  false,
		},
		// ----------- Stdin -----------
		{
			Name:             "stdin false value restricted by any value",
			Override:         corev1.Container{Stdin: false},
			RestrictedFields: []string{"stdin"},
			IsErrorExpected:  true,
			ErrField:         "stdin",
		},
		{
			Name:             "stdin restricted by specific value match",
			Override:         corev1.Container{Stdin: true},
			RestrictedFields: []string{"stdin=true"},
			IsErrorExpected:  true,
			ErrField:         "stdin=true",
		},
		{
			Name:             "stdin restricted by any value",
			Override:         corev1.Container{Stdin: true},
			RestrictedFields: []string{"stdin"},
			IsErrorExpected:  true,
			ErrField:         "stdin",
		},
		{
			Name:             "stdin allowed when restricted value does not match",
			Override:         corev1.Container{Stdin: true},
			RestrictedFields: []string{"stdin=false"},
			IsErrorExpected:  false,
		},
		// ----------- StdinOnce -----------
		{
			Name:             "stdinOnce false value restricted by any value",
			Override:         corev1.Container{StdinOnce: false},
			RestrictedFields: []string{"stdinOnce"},
			IsErrorExpected:  true,
			ErrField:         "stdinOnce",
		},
		{
			Name:             "stdinOnce restricted by specific value match",
			Override:         corev1.Container{StdinOnce: true},
			RestrictedFields: []string{"stdinOnce=true"},
			IsErrorExpected:  true,
			ErrField:         "stdinOnce=true",
		},
		{
			Name:             "stdinOnce restricted by any value",
			Override:         corev1.Container{StdinOnce: true},
			RestrictedFields: []string{"stdinOnce"},
			IsErrorExpected:  true,
			ErrField:         "stdinOnce",
		},
		{
			Name:             "stdinOnce allowed when restricted value does not match",
			Override:         corev1.Container{StdinOnce: true},
			RestrictedFields: []string{"stdinOnce=false"},
			IsErrorExpected:  false,
		},
		// ----------- TTY -----------
		{
			Name:             "tty false value restricted by any value",
			Override:         corev1.Container{TTY: false},
			RestrictedFields: []string{"tty"},
			IsErrorExpected:  true,
			ErrField:         "tty",
		},
		{
			Name:             "tty restricted by specific value match",
			Override:         corev1.Container{TTY: true},
			RestrictedFields: []string{"tty=true"},
			IsErrorExpected:  true,
			ErrField:         "tty=true",
		},
		{
			Name:             "tty restricted by any value",
			Override:         corev1.Container{TTY: true},
			RestrictedFields: []string{"tty"},
			IsErrorExpected:  true,
			ErrField:         "tty",
		},
		{
			Name:             "tty allowed when restricted value does not match",
			Override:         corev1.Container{TTY: true},
			RestrictedFields: []string{"tty=false"},
			IsErrorExpected:  false,
		},
		// ----------- EnvFrom -----------
		{
			Name:             "envFrom empty slice not restricted",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{}},
			RestrictedFields: []string{"envFrom"},
			IsErrorExpected:  false,
		},
		{
			Name:             "envFrom restricted by any value",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{{}}},
			RestrictedFields: []string{"envFrom"},
			IsErrorExpected:  true,
			ErrField:         "envFrom",
		},
		{
			Name:             "envFrom.configMapRef nil value not restricted",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: nil}}},
			RestrictedFields: []string{"envFrom.configMapRef"},
			IsErrorExpected:  false,
		},
		{
			Name:             "envFrom.secretRef nil value not restricted",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{{SecretRef: nil}}},
			RestrictedFields: []string{"envFrom.secretRef"},
			IsErrorExpected:  false,
		},
		{
			Name:             "envFrom.configMapRef restricted by any value",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{}}}},
			RestrictedFields: []string{"envFrom.configMapRef"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.configMapRef",
		},
		{
			Name:             "envFrom.secretRef restricted by any value",
			Override:         corev1.Container{EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{}}}},
			RestrictedFields: []string{"envFrom.secretRef"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.secretRef",
		},
		{
			Name: "envFrom.configMapRef.name restricted by any value",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"}},
			}}},
			RestrictedFields: []string{"envFrom.configMapRef.name"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.configMapRef.name",
		},
		{
			Name: "envFrom.secretRef.name restricted by any value",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"}},
			}}},
			RestrictedFields: []string{"envFrom.secretRef.name"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.secretRef.name",
		},
		{
			Name: "envFrom.configMapRef.name restricted by specific value match",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"}},
			}}},
			RestrictedFields: []string{"envFrom.configMapRef.name=my-config"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.configMapRef.name=my-config",
		},
		{
			Name: "envFrom.secretRef.name restricted by specific value match",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"}},
			}}},
			RestrictedFields: []string{"envFrom.secretRef.name=my-secret"},
			IsErrorExpected:  true,
			ErrField:         "envFrom.secretRef.name=my-secret",
		},
		{
			Name: "envFrom.configMapRef.name allowed when restricted value does not match",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"}},
			}}},
			RestrictedFields: []string{"envFrom.configMapRef.name=other-config"},
			IsErrorExpected:  false,
		},
		{
			Name: "envFrom.secretRef.name allowed when restricted value does not match",
			Override: corev1.Container{EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"}},
			}}},
			RestrictedFields: []string{"envFrom.secretRef.name=other-secret"},
			IsErrorExpected:  false,
		},
		// ----------- RestartPolicyRules -----------
		{
			Name:             "restartPolicyRules empty slice not restricted",
			Override:         corev1.Container{RestartPolicyRules: nil},
			RestrictedFields: []string{"restartPolicyRules"},
			IsErrorExpected:  false,
		},
		{
			Name:             "restartPolicyRules restricted by any value",
			Override:         corev1.Container{RestartPolicyRules: []corev1.ContainerRestartRule{{}}},
			RestrictedFields: []string{"restartPolicyRules"},
			IsErrorExpected:  true,
			ErrField:         "restartPolicyRules",
		},
		// ----------- ResizePolicy -----------
		{
			Name:             "resizePolicy empty slice not restricted",
			Override:         corev1.Container{ResizePolicy: nil},
			RestrictedFields: []string{"resizePolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resizePolicy restricted by any value",
			Override:         corev1.Container{ResizePolicy: []corev1.ContainerResizePolicy{{}}},
			RestrictedFields: []string{"resizePolicy"},
			IsErrorExpected:  true,
			ErrField:         "resizePolicy",
		},
		// ----------- ReadinessProbe -----------
		{
			Name:             "readinessProbe nil not restricted",
			Override:         corev1.Container{ReadinessProbe: nil},
			RestrictedFields: []string{"readinessProbe"},
			IsErrorExpected:  false,
		},
		{
			Name:             "readinessProbe restricted by any value",
			Override:         corev1.Container{ReadinessProbe: &corev1.Probe{}},
			RestrictedFields: []string{"readinessProbe"},
			IsErrorExpected:  true,
			ErrField:         "readinessProbe",
		},
		// ----------- StartupProbe -----------
		{
			Name:             "startupProbe nil not restricted",
			Override:         corev1.Container{StartupProbe: nil},
			RestrictedFields: []string{"startupProbe"},
			IsErrorExpected:  false,
		},
		{
			Name:             "startupProbe restricted by any value",
			Override:         corev1.Container{StartupProbe: &corev1.Probe{}},
			RestrictedFields: []string{"startupProbe"},
			IsErrorExpected:  true,
			ErrField:         "startupProbe",
		},
		// ----------- LivenessProbe -----------
		{
			Name:             "livenessProbe nil not restricted",
			Override:         corev1.Container{LivenessProbe: nil},
			RestrictedFields: []string{"livenessProbe"},
			IsErrorExpected:  false,
		},
		{
			Name:             "livenessProbe restricted by any value",
			Override:         corev1.Container{LivenessProbe: &corev1.Probe{}},
			RestrictedFields: []string{"livenessProbe"},
			IsErrorExpected:  true,
			ErrField:         "livenessProbe",
		},
		// ----------- Lifecycle -----------
		{
			Name:             "lifecycle nil not restricted",
			Override:         corev1.Container{Lifecycle: nil},
			RestrictedFields: []string{"lifecycle"},
			IsErrorExpected:  false,
		},
		{
			Name:             "lifecycle restricted by any value",
			Override:         corev1.Container{Lifecycle: &corev1.Lifecycle{}},
			RestrictedFields: []string{"lifecycle"},
			IsErrorExpected:  true,
			ErrField:         "lifecycle",
		},
		// ----------- Resources -----------
		{
			Name:             "resources empty not restricted",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{}},
			RestrictedFields: []string{"resources"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{}}},
			RestrictedFields: []string{"resources"},
			IsErrorExpected:  true,
			ErrField:         "resources",
		},
		// ----------- Resources.Limits -----------
		{
			Name:             "resources.limits nil not restricted",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Limits: nil}},
			RestrictedFields: []string{"resources.limits"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.limits restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{}}},
			RestrictedFields: []string{"resources.limits"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits",
		},
		// ----------- Resources.Requests -----------
		{
			Name:             "resources.requests nil not restricted",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Requests: nil}},
			RestrictedFields: []string{"resources.requests"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.requests restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{}}},
			RestrictedFields: []string{"resources.requests"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests",
		},
		// ----------- Resources.Claims -----------
		{
			Name:             "resources.claims empty not restricted",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Requests: nil}},
			RestrictedFields: []string{"resources.claims"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.claims restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu"}}}},
			RestrictedFields: []string{"resources.claims"},
			IsErrorExpected:  true,
			ErrField:         "resources.claims",
		},
		// ----------- Resources.Limits.CPU -----------
		{
			Name:             "resources.limits.cpu restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}}},
			RestrictedFields: []string{"resources.limits.cpu"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits.cpu",
		},
		// ----------- Resources.Limits.Memory -----------
		{
			Name:             "resources.limits.memory restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")}}},
			RestrictedFields: []string{"resources.limits.memory"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits.memory",
		},
		// ----------- Resources.Requests.CPU -----------
		{
			Name:             "resources.requests.cpu restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")}}},
			RestrictedFields: []string{"resources.requests.cpu"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests.cpu",
		},
		// ----------- Resources.Requests.Memory -----------
		{
			Name:             "resources.requests.memory restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}}},
			RestrictedFields: []string{"resources.requests.memory"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests.memory",
		},
		// ----------- Resources.Claims.Request -----------
		{
			Name:             "resources.claims.request empty claims not restricted",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{}}},
			RestrictedFields: []string{"resources.claims.request"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.claims.request restricted by specific value match",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu", Request: "gpu-partition"}}}},
			RestrictedFields: []string{"resources.claims.request=gpu-partition"},
			IsErrorExpected:  true,
			ErrField:         "resources.claims.request=gpu-partition",
		},
		{
			Name:             "resources.claims.request restricted by any value",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu", Request: "gpu-partition"}}}},
			RestrictedFields: []string{"resources.claims.request"},
			IsErrorExpected:  true,
			ErrField:         "resources.claims.request",
		},
		{
			Name:             "resources.claims.request allowed when restricted value does not match",
			Override:         corev1.Container{Resources: corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu", Request: "gpu-partition"}}}},
			RestrictedFields: []string{"resources.claims.request=other-partition"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts -----------
		{
			Name:             "volumeMounts empty slice not restricted",
			Override:         corev1.Container{VolumeMounts: nil},
			RestrictedFields: []string{"volumeMounts"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{}}},
			RestrictedFields: []string{"volumeMounts"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts",
		},
		// ----------- VolumeMounts.ReadOnly -----------
		{
			Name:             "volumeMounts.readOnly false value restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{ReadOnly: false}}},
			RestrictedFields: []string{"volumeMounts.readOnly"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.readOnly",
		},
		{
			Name:             "volumeMounts.readOnly restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{ReadOnly: true}}},
			RestrictedFields: []string{"volumeMounts.readOnly=true"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.readOnly=true",
		},
		{
			Name:             "volumeMounts.readOnly restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{ReadOnly: true}}},
			RestrictedFields: []string{"volumeMounts.readOnly"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.readOnly",
		},
		{
			Name:             "volumeMounts.readOnly allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{ReadOnly: true}}},
			RestrictedFields: []string{"volumeMounts.readOnly=false"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.RecursiveReadOnly -----------
		{
			Name:             "volumeMounts.recursiveReadOnly nil value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{RecursiveReadOnly: nil}}},
			RestrictedFields: []string{"volumeMounts.recursiveReadOnly"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.recursiveReadOnly restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{RecursiveReadOnly: ptr.To(corev1.RecursiveReadOnlyEnabled)}}},
			RestrictedFields: []string{"volumeMounts.recursiveReadOnly=Enabled"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.recursiveReadOnly=Enabled",
		},
		{
			Name:             "volumeMounts.recursiveReadOnly restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{RecursiveReadOnly: ptr.To(corev1.RecursiveReadOnlyEnabled)}}},
			RestrictedFields: []string{"volumeMounts.recursiveReadOnly"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.recursiveReadOnly",
		},
		{
			Name:             "volumeMounts.recursiveReadOnly allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{RecursiveReadOnly: ptr.To(corev1.RecursiveReadOnlyEnabled)}}},
			RestrictedFields: []string{"volumeMounts.recursiveReadOnly=Disabled"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.MountPropagation -----------
		{
			Name:             "volumeMounts.mountPropagation nil value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPropagation: nil}}},
			RestrictedFields: []string{"volumeMounts.mountPropagation"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.mountPropagation restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPropagation: ptr.To(corev1.MountPropagationBidirectional)}}},
			RestrictedFields: []string{"volumeMounts.mountPropagation=Bidirectional"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.mountPropagation=Bidirectional",
		},
		{
			Name:             "volumeMounts.mountPropagation restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPropagation: ptr.To(corev1.MountPropagationBidirectional)}}},
			RestrictedFields: []string{"volumeMounts.mountPropagation"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.mountPropagation",
		},
		{
			Name:             "volumeMounts.mountPropagation allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPropagation: ptr.To(corev1.MountPropagationBidirectional)}}},
			RestrictedFields: []string{"volumeMounts.mountPropagation=HostToContainer"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.Name -----------
		{
			Name:             "volumeMounts.name empty value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{Name: ""}}},
			RestrictedFields: []string{"volumeMounts.name"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.name restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumeMounts.name=my-volume"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.name=my-volume",
		},
		{
			Name:             "volumeMounts.name restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumeMounts.name"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.name",
		},
		{
			Name:             "volumeMounts.name allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumeMounts.name=other-volume"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.MountPath -----------
		{
			Name:             "volumeMounts.mountPath empty value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPath: ""}}},
			RestrictedFields: []string{"volumeMounts.mountPath"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.mountPath restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPath: "/data"}}},
			RestrictedFields: []string{"volumeMounts.mountPath=/data"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.mountPath=/data",
		},
		{
			Name:             "volumeMounts.mountPath restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPath: "/data"}}},
			RestrictedFields: []string{"volumeMounts.mountPath"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.mountPath",
		},
		{
			Name:             "volumeMounts.mountPath allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{MountPath: "/data"}}},
			RestrictedFields: []string{"volumeMounts.mountPath=/other"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.SubPath -----------
		{
			Name:             "volumeMounts.subPath empty value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPath: ""}}},
			RestrictedFields: []string{"volumeMounts.subPath"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.subPath restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPath: "config"}}},
			RestrictedFields: []string{"volumeMounts.subPath=config"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.subPath=config",
		},
		{
			Name:             "volumeMounts.subPath restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPath: "config"}}},
			RestrictedFields: []string{"volumeMounts.subPath"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.subPath",
		},
		{
			Name:             "volumeMounts.subPath allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPath: "config"}}},
			RestrictedFields: []string{"volumeMounts.subPath=other"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeMounts.SubPathExpr -----------
		{
			Name:             "volumeMounts.subPathExpr empty value not restricted",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPathExpr: ""}}},
			RestrictedFields: []string{"volumeMounts.subPathExpr"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeMounts.subPathExpr restricted by specific value match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPathExpr: "$(POD_NAME)"}}},
			RestrictedFields: []string{"volumeMounts.subPathExpr=$(POD_NAME)"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.subPathExpr=$(POD_NAME)",
		},
		{
			Name:             "volumeMounts.subPathExpr restricted by any value",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPathExpr: "$(POD_NAME)"}}},
			RestrictedFields: []string{"volumeMounts.subPathExpr"},
			IsErrorExpected:  true,
			ErrField:         "volumeMounts.subPathExpr",
		},
		{
			Name:             "volumeMounts.subPathExpr allowed when restricted value does not match",
			Override:         corev1.Container{VolumeMounts: []corev1.VolumeMount{{SubPathExpr: "$(POD_NAME)"}}},
			RestrictedFields: []string{"volumeMounts.subPathExpr=$(OTHER)"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeDevices -----------
		{
			Name:             "volumeDevices empty slice not restricted",
			Override:         corev1.Container{VolumeDevices: nil},
			RestrictedFields: []string{"volumeDevices"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeDevices restricted by any value",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{}}},
			RestrictedFields: []string{"volumeDevices"},
			IsErrorExpected:  true,
			ErrField:         "volumeDevices",
		},
		// ----------- VolumeDevices.Name -----------
		{
			Name:             "volumeDevices.name empty value not restricted",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{Name: ""}}},
			RestrictedFields: []string{"volumeDevices.name"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeDevices.name restricted by specific value match",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{Name: "my-device"}}},
			RestrictedFields: []string{"volumeDevices.name=my-device"},
			IsErrorExpected:  true,
			ErrField:         "volumeDevices.name=my-device",
		},
		{
			Name:             "volumeDevices.name restricted by any value",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{Name: "my-device"}}},
			RestrictedFields: []string{"volumeDevices.name"},
			IsErrorExpected:  true,
			ErrField:         "volumeDevices.name",
		},
		{
			Name:             "volumeDevices.name allowed when restricted value does not match",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{Name: "my-device"}}},
			RestrictedFields: []string{"volumeDevices.name=other-device"},
			IsErrorExpected:  false,
		},
		// ----------- VolumeDevices.DevicePath -----------
		{
			Name:             "volumeDevices.devicePath empty value not restricted",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{DevicePath: ""}}},
			RestrictedFields: []string{"volumeDevices.devicePath"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumeDevices.devicePath restricted by specific value match",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{DevicePath: "/dev/sda"}}},
			RestrictedFields: []string{"volumeDevices.devicePath=/dev/sda"},
			IsErrorExpected:  true,
			ErrField:         "volumeDevices.devicePath=/dev/sda",
		},
		{
			Name:             "volumeDevices.devicePath restricted by any value",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{DevicePath: "/dev/sda"}}},
			RestrictedFields: []string{"volumeDevices.devicePath"},
			IsErrorExpected:  true,
			ErrField:         "volumeDevices.devicePath",
		},
		{
			Name:             "volumeDevices.devicePath allowed when restricted value does not match",
			Override:         corev1.Container{VolumeDevices: []corev1.VolumeDevice{{DevicePath: "/dev/sda"}}},
			RestrictedFields: []string{"volumeDevices.devicePath=/dev/sdb"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext  -----------
		{
			Name:             "securityContext nil not restricted",
			Override:         corev1.Container{SecurityContext: nil},
			RestrictedFields: []string{"securityContext"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{}},
			RestrictedFields: []string{"securityContext"},
			IsErrorExpected:  true,
			ErrField:         "securityContext",
		},
		// ----------- SecurityContext.SELinuxOptions -----------
		{
			Name:             "securityContext.seLinuxOptions nil not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{SELinuxOptions: nil}},
			RestrictedFields: []string{"securityContext.seLinuxOptions"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.seLinuxOptions restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{SELinuxOptions: &corev1.SELinuxOptions{}}},
			RestrictedFields: []string{"securityContext.seLinuxOptions"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seLinuxOptions",
		},
		// ----------- SecurityContext.SeccompProfile -----------
		{
			Name:             "securityContext.seccompProfile nil not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{SeccompProfile: nil}},
			RestrictedFields: []string{"securityContext.seccompProfile"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.seccompProfile restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{SeccompProfile: &corev1.SeccompProfile{}}},
			RestrictedFields: []string{"securityContext.seccompProfile"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seccompProfile",
		},
		// ----------- SecurityContext.AppArmorProfile -----------
		{
			Name:             "securityContext.appArmorProfile nil not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AppArmorProfile: nil}},
			RestrictedFields: []string{"securityContext.appArmorProfile"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.appArmorProfile restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AppArmorProfile: &corev1.AppArmorProfile{}}},
			RestrictedFields: []string{"securityContext.appArmorProfile"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.appArmorProfile",
		},
		// ----------- SecurityContext.Privileged -----------
		{
			Name:             "securityContext.privileged nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Privileged: nil}},
			RestrictedFields: []string{"securityContext.privileged"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.privileged restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.privileged=true"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.privileged=true",
		},
		{
			Name:             "securityContext.privileged restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.privileged"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.privileged",
		},
		{
			Name:             "securityContext.privileged allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.privileged=false"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.RunAsNonRoot -----------
		{
			Name:             "securityContext.runAsNonRoot nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsNonRoot: nil}},
			RestrictedFields: []string{"securityContext.runAsNonRoot"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsNonRoot restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot=true"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsNonRoot=true",
		},
		{
			Name:             "securityContext.runAsNonRoot restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsNonRoot",
		},
		{
			Name:             "securityContext.runAsNonRoot allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot=false"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.ReadOnlyRootFilesystem -----------
		{
			Name:             "securityContext.readOnlyRootFilesystem nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: nil}},
			RestrictedFields: []string{"securityContext.readOnlyRootFilesystem"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.readOnlyRootFilesystem restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.readOnlyRootFilesystem=true"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.readOnlyRootFilesystem=true",
		},
		{
			Name:             "securityContext.readOnlyRootFilesystem restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.readOnlyRootFilesystem"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.readOnlyRootFilesystem",
		},
		{
			Name:             "securityContext.readOnlyRootFilesystem allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.readOnlyRootFilesystem=false"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.AllowPrivilegeEscalation -----------
		{
			Name:             "securityContext.allowPrivilegeEscalation nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: nil}},
			RestrictedFields: []string{"securityContext.allowPrivilegeEscalation"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.allowPrivilegeEscalation restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.allowPrivilegeEscalation=true"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.allowPrivilegeEscalation=true",
		},
		{
			Name:             "securityContext.allowPrivilegeEscalation restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.allowPrivilegeEscalation"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.allowPrivilegeEscalation",
		},
		{
			Name:             "securityContext.allowPrivilegeEscalation allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.allowPrivilegeEscalation=false"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.ProcMount -----------
		{
			Name:             "securityContext.procMount nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ProcMount: nil}},
			RestrictedFields: []string{"securityContext.procMount"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.procMount restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ProcMount: ptr.To(corev1.UnmaskedProcMount)}},
			RestrictedFields: []string{"securityContext.procMount=Unmasked"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.procMount=Unmasked",
		},
		{
			Name:             "securityContext.procMount restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ProcMount: ptr.To(corev1.UnmaskedProcMount)}},
			RestrictedFields: []string{"securityContext.procMount"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.procMount",
		},
		{
			Name:             "securityContext.procMount allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{ProcMount: ptr.To(corev1.UnmaskedProcMount)}},
			RestrictedFields: []string{"securityContext.procMount=Default"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.RunAsUser -----------
		{
			Name:             "securityContext.runAsUser nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsUser: nil}},
			RestrictedFields: []string{"securityContext.runAsUser"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsUser restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser=1000"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsUser=1000",
		},
		{
			Name:             "securityContext.runAsUser restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsUser",
		},
		{
			Name:             "securityContext.runAsUser allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser=2000"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.RunAsGroup -----------
		{
			Name:             "securityContext.runAsGroup nil value not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsGroup: nil}},
			RestrictedFields: []string{"securityContext.runAsGroup"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsGroup restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup=500"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsGroup=500",
		},
		{
			Name:             "securityContext.runAsGroup restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsGroup",
		},
		{
			Name:             "securityContext.runAsGroup allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup=1000"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.Capabilities  -----------
		{
			Name:             "securityContext.capabilities nil not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: nil}},
			RestrictedFields: []string{"securityContext.capabilities"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.capabilities restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{}}},
			RestrictedFields: []string{"securityContext.capabilities"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.capabilities",
		},
		// ----------- SecurityContext.Capabilities.Add -----------
		{
			Name:             "securityContext.capabilities.add nil capabilities not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Add: []corev1.Capability{}}}},
			RestrictedFields: []string{"securityContext.capabilities.add"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.capabilities.add restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN"}}}},
			RestrictedFields: []string{"securityContext.capabilities.add=NET_ADMIN"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.capabilities.add=NET_ADMIN",
		},
		{
			Name:             "securityContext.capabilities.add restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN"}}}},
			RestrictedFields: []string{"securityContext.capabilities.add"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.capabilities.add",
		},
		{
			Name:             "securityContext.capabilities.add allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN"}}}},
			RestrictedFields: []string{"securityContext.capabilities.add=SYS_ADMIN"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.Capabilities.Drop -----------
		{
			Name:             "securityContext.capabilities.drop nil capabilities not restricted",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{}}}},
			RestrictedFields: []string{"securityContext.capabilities.drop"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.capabilities.drop restricted by specific value match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}},
			RestrictedFields: []string{"securityContext.capabilities.drop=ALL"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.capabilities.drop=ALL",
		},
		{
			Name:             "securityContext.capabilities.drop restricted by any value",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}},
			RestrictedFields: []string{"securityContext.capabilities.drop"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.capabilities.drop",
		},
		{
			Name:             "securityContext.capabilities.drop allowed when restricted value does not match",
			Override:         corev1.Container{SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}},
			RestrictedFields: []string{"securityContext.capabilities.drop=NET_RAW"},
			IsErrorExpected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			err := restrictContainerOverride(&tt.Override, tt.RestrictedFields)

			if tt.IsErrorExpected {
				assert.Error(t, err)
				assert.Equal(t, getContainerRestrictionErr(tt.ErrField).Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyContainerOverridesStripsUnknownFields(t *testing.T) {
	overrideJSON := `{"workingDir":"/workspace","futureSecurityField":"malicious-value","unknownNested":{"key":"val"}}`

	component := &dw.Component{
		Name: "test-component",
		Attributes: attributes.Attributes{
			constants.ContainerOverridesAttribute: apiext.JSON{Raw: []byte(overrideJSON)},
		},
		ComponentUnion: dw.ComponentUnion{
			Container: &dw.ContainerComponent{
				Container: dw.Container{Image: "test-image"},
			},
		},
	}
	container := &corev1.Container{
		Name:  "test-component",
		Image: "test-image",
	}

	patched, err := ApplyContainerOverrides(component, container, nil)
	assert.NoError(t, err)
	assert.Equal(t, "/workspace", patched.WorkingDir)

	patchedBytes, err := json.Marshal(patched)
	assert.NoError(t, err)
	assert.NotContains(t, string(patchedBytes), "futureSecurityField")
	assert.NotContains(t, string(patchedBytes), "unknownNested")
}
