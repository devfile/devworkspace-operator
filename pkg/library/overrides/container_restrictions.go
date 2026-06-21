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
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func getContainerRestrictionErr(msg string) error {
	return fmt.Errorf("cannot use container-overrides to override container %s", msg)
}

func restrictContainerOverride(override *corev1.Container, restrictedFields []string) error {
	if override.Name != "" {
		return getContainerRestrictionErr("name")
	}
	if override.Image != "" {
		return getContainerRestrictionErr("image")
	}
	if override.Command != nil {
		return getContainerRestrictionErr("command")
	}
	if override.Args != nil {
		return getContainerRestrictionErr("args")
	}
	if override.Ports != nil {
		return getContainerRestrictionErr("ports")
	}
	if override.Env != nil {
		return getContainerRestrictionErr("env")
	}

	for _, restrictedField := range restrictedFields {
		fieldName, fieldValue, _ := strings.Cut(restrictedField, "=")
		if fieldName == "" {
			continue
		}

		root, remaining, _ := strings.Cut(fieldName, ".")

		restriction := &FieldRestriction{
			name:              fieldName,
			restrictedValue:   fieldValue,
			getRestrictionErr: getContainerRestrictionErr,
		}

		if err := checkContainer(override, root, remaining, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkContainer(override *corev1.Container, root string, remaining string, restriction *FieldRestriction) error {
	if remaining == "" {
		switch root {
		case "workingDir":
			return restriction.checkString(&override.WorkingDir)
		case "restartPolicy":
			return restriction.checkString((*string)(override.RestartPolicy))
		case "terminationMessagePath":
			return restriction.checkString(&override.TerminationMessagePath)
		case "terminationMessagePolicy":
			return restriction.checkString((*string)(&override.TerminationMessagePolicy))
		case "imagePullPolicy":
			return restriction.checkString((*string)(&override.ImagePullPolicy))
		case "stdin":
			return restriction.checkBool(&override.Stdin)
		case "stdinOnce":
			return restriction.checkBool(&override.StdinOnce)
		case "tty":
			return restriction.checkBool(&override.TTY)
		case "envFrom":
			if len(override.EnvFrom) != 0 {
				return restriction.checkAny()
			}
		case "restartPolicyRules":
			if len(override.RestartPolicyRules) != 0 {
				return restriction.checkAny()
			}
		case "resizePolicy":
			if len(override.ResizePolicy) != 0 {
				return restriction.checkAny()
			}
		case "readinessProbe":
			if override.ReadinessProbe != nil {
				return restriction.checkAny()
			}
		case "startupProbe":
			if override.StartupProbe != nil {
				return restriction.checkAny()
			}
		case "livenessProbe":
			if override.LivenessProbe != nil {
				return restriction.checkAny()
			}
		case "lifecycle":
			if override.Lifecycle != nil {
				return restriction.checkAny()
			}
		case "resources":
			if !reflect.DeepEqual(override.Resources, corev1.ResourceRequirements{}) {
				return restriction.checkAny()
			}
		case "volumeMounts":
			if len(override.VolumeMounts) > 0 {
				return restriction.checkAny()
			}
		case "volumeDevices":
			if len(override.VolumeDevices) > 0 {
				return restriction.checkAny()
			}
		case "securityContext":
			if override.SecurityContext != nil {
				return restriction.checkAny()
			}
		}

		return nil
	}

	switch root {
	case "envFrom":
		return checkEnvsFrom(override.EnvFrom, remaining, restriction)
	case "resources":
		return checkResources(&override.Resources, remaining, restriction)
	case "volumeMounts":
		return checkVolumeMounts(override.VolumeMounts, remaining, restriction)
	case "volumeDevices":
		return checkVolumeDevices(override.VolumeDevices, remaining, restriction)
	case "securityContext":
		return checkContainerSecurityContext(override.SecurityContext, remaining, restriction)
	}

	return nil
}

func checkEnvsFrom(envsFrom []corev1.EnvFromSource, field string, restriction *FieldRestriction) error {
	if len(envsFrom) == 0 {
		return nil
	}

	root, remaining, _ := strings.Cut(field, ".")

	for _, envFrom := range envsFrom {
		if err := checkEnvFrom(envFrom, root, remaining, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkEnvFrom(envFrom corev1.EnvFromSource, root string, remaining string, restriction *FieldRestriction) error {
	if remaining == "" {
		switch root {
		case "configMapRef":
			if envFrom.ConfigMapRef != nil {
				return restriction.checkAny()
			}
		case "secretRef":
			if envFrom.SecretRef != nil {
				return restriction.checkAny()
			}
		}
		return nil
	}

	switch root {
	case "configMapRef":
		return checkConfigMapRef(envFrom.ConfigMapRef, remaining, restriction)
	case "secretRef":
		return checkSecretRef(envFrom.SecretRef, remaining, restriction)
	}

	return nil
}

func checkConfigMapRef(cmRef *corev1.ConfigMapEnvSource, field string, restriction *FieldRestriction) error {
	if cmRef == nil {
		return nil
	}

	switch field {
	case "name":
		return restriction.checkString(&cmRef.Name)
	}

	return nil
}

func checkSecretRef(secretRef *corev1.SecretEnvSource, field string, restriction *FieldRestriction) error {
	if secretRef == nil {
		return nil
	}

	switch field {
	case "name":
		return restriction.checkString(&secretRef.Name)
	}

	return nil
}

func checkVolumeMounts(mounts []corev1.VolumeMount, field string, restriction *FieldRestriction) error {
	if len(mounts) == 0 {
		return nil
	}

	for _, mount := range mounts {
		if err := checkVolumeMount(mount, field, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkVolumeMount(mount corev1.VolumeMount, field string, restriction *FieldRestriction) error {
	switch field {
	case "readOnly":
		return restriction.checkBool(&mount.ReadOnly)
	case "recursiveReadOnly":
		return restriction.checkString((*string)(mount.RecursiveReadOnly))
	case "mountPropagation":
		return restriction.checkString((*string)(mount.MountPropagation))
	case "name":
		return restriction.checkString(&mount.Name)
	case "mountPath":
		return restriction.checkString(&mount.MountPath)
	case "subPath":
		return restriction.checkString(&mount.SubPath)
	case "subPathExpr":
		return restriction.checkString(&mount.SubPathExpr)
	}

	return nil
}

func checkVolumeDevices(devices []corev1.VolumeDevice, field string, restriction *FieldRestriction) error {
	if len(devices) == 0 {
		return nil
	}

	for _, device := range devices {
		if err := checkVolumeDevice(device, field, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkVolumeDevice(device corev1.VolumeDevice, field string, restriction *FieldRestriction) error {
	switch field {
	case "name":
		return restriction.checkString(&device.Name)
	case "devicePath":
		return restriction.checkString(&device.DevicePath)
	}

	return nil
}

func checkContainerSecurityContext(sc *corev1.SecurityContext, field string, restriction *FieldRestriction) error {
	if sc == nil {
		return nil
	}

	root, remaining, _ := strings.Cut(field, ".")

	if remaining == "" {
		switch root {
		case "capabilities":
			if sc.Capabilities != nil {
				return restriction.checkAny()
			}
		case "privileged":
			return restriction.checkBool(sc.Privileged)
		case "seLinuxOptions":
			if sc.SELinuxOptions != nil {
				return restriction.checkAny()
			}
		case "runAsUser":
			return restriction.checkInt64(sc.RunAsUser)
		case "runAsGroup":
			return restriction.checkInt64(sc.RunAsGroup)
		case "runAsNonRoot":
			return restriction.checkBool(sc.RunAsNonRoot)
		case "readOnlyRootFilesystem":
			return restriction.checkBool(sc.ReadOnlyRootFilesystem)
		case "allowPrivilegeEscalation":
			return restriction.checkBool(sc.AllowPrivilegeEscalation)
		case "procMount":
			return restriction.checkString((*string)(sc.ProcMount))
		case "seccompProfile":
			if sc.SeccompProfile != nil {
				return restriction.checkAny()
			}
		case "appArmorProfile":
			if sc.AppArmorProfile != nil {
				return restriction.checkAny()
			}
		}

		return nil
	}

	switch root {
	case "capabilities":
		return checkCapabilities(sc.Capabilities, remaining, restriction)
	}

	return nil
}

func checkCapabilities(caps *corev1.Capabilities, field string, restriction *FieldRestriction) error {
	if caps == nil {
		return nil
	}

	switch field {
	case "add":
		for _, capAdd := range caps.Add {
			if err := restriction.checkString((*string)(&capAdd)); err != nil {
				return err
			}
		}
	case "drop":
		for _, capDrop := range caps.Drop {
			if err := restriction.checkString((*string)(&capDrop)); err != nil {
				return err
			}
		}
	}

	return nil
}
