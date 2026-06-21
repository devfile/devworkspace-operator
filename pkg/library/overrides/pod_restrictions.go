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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func getPodRestrictionErr(msg string) error {
	return fmt.Errorf("cannot use pod-overrides to override pod %s", msg)
}

func restrictPodOverride(override *corev1.PodSpec, restrictedFields []string) error {
	if override.Containers != nil {
		return getPodRestrictionErr("containers")
	}
	if override.InitContainers != nil {
		return getPodRestrictionErr("initContainers")
	}

	for _, field := range restrictedFields {
		fieldName, fieldValue, _ := strings.Cut(field, "=")
		if fieldName == "" {
			continue
		}

		root, remaining, _ := strings.Cut(fieldName, ".")

		restriction := &FieldRestriction{
			name:              fieldName,
			restrictedValue:   fieldValue,
			getRestrictionErr: getPodRestrictionErr,
		}

		if err := checkPodField(override, root, remaining, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkPodField(override *corev1.PodSpec, root string, remaining string, restriction *FieldRestriction) error {
	if remaining == "" {
		switch root {
		case "restartPolicy":
			return restriction.checkString((*string)(&override.RestartPolicy))
		case "terminationGracePeriodSeconds":
			return restriction.checkInt64(override.TerminationGracePeriodSeconds)
		case "activeDeadlineSeconds":
			return restriction.checkInt64(override.ActiveDeadlineSeconds)
		case "dnsPolicy":
			return restriction.checkString((*string)(&override.DNSPolicy))
		case "nodeSelector":
			if len(override.NodeSelector) > 0 {
				return restriction.checkAny()
			}
		case "serviceAccountName":
			return restriction.checkString(&override.ServiceAccountName)
		case "deprecatedServiceAccount":
			return restriction.checkString(&override.DeprecatedServiceAccount)
		case "automountServiceAccountToken":
			return restriction.checkBool(override.AutomountServiceAccountToken)
		case "nodeName":
			return restriction.checkString(&override.NodeName)
		case "hostIPC":
			return restriction.checkBool(&override.HostIPC)
		case "hostPID":
			return restriction.checkBool(&override.HostPID)
		case "hostNetwork":
			return restriction.checkBool(&override.HostNetwork)
		case "shareProcessNamespace":
			return restriction.checkBool(override.ShareProcessNamespace)
		case "hostname":
			return restriction.checkString(&override.Hostname)
		case "subdomain":
			return restriction.checkString(&override.Subdomain)
		case "schedulerName":
			return restriction.checkString(&override.SchedulerName)
		case "priorityClassName":
			return restriction.checkString(&override.PriorityClassName)
		case "priority":
			return restriction.checkInt32(override.Priority)
		case "runtimeClassName":
			return restriction.checkString(override.RuntimeClassName)
		case "enableServiceLinks":
			return restriction.checkBool(override.EnableServiceLinks)
		case "preemptionPolicy":
			return restriction.checkString((*string)(override.PreemptionPolicy))
		case "setHostnameAsFQDN":
			return restriction.checkBool(override.SetHostnameAsFQDN)
		case "hostUsers":
			return restriction.checkBool(override.HostUsers)
		case "hostnameOverride":
			return restriction.checkString(override.HostnameOverride)
		case "ephemeralContainers":
			if len(override.EphemeralContainers) > 0 {
				return restriction.checkAny()
			}
		case "affinity":
			if override.Affinity != nil {
				return restriction.checkAny()
			}
		case "tolerations":
			if len(override.Tolerations) > 0 {
				return restriction.checkAny()
			}
		case "hostAliases":
			if len(override.HostAliases) > 0 {
				return restriction.checkAny()
			}
		case "dnsConfig":
			if override.DNSConfig != nil {
				return restriction.checkAny()
			}
		case "readinessGates":
			if len(override.ReadinessGates) > 0 {
				return restriction.checkAny()
			}
		case "topologySpreadConstraints":
			if len(override.TopologySpreadConstraints) > 0 {
				return restriction.checkAny()
			}
		case "os":
			if override.OS != nil {
				return restriction.checkAny()
			}
		case "schedulingGates":
			if len(override.SchedulingGates) > 0 {
				return restriction.checkAny()
			}
		case "workloadRef":
			if override.WorkloadRef != nil {
				return restriction.checkAny()
			}
		case "volumes":
			if len(override.Volumes) > 0 {
				return restriction.checkAny()
			}
		case "securityContext":
			if override.SecurityContext != nil {
				return restriction.checkAny()
			}
		case "imagePullSecrets":
			if len(override.ImagePullSecrets) > 0 {
				return restriction.checkAny()
			}
		case "overhead":
			if len(override.Overhead) > 0 {
				return restriction.checkAny()
			}
		case "resourceClaims":
			if len(override.ResourceClaims) > 0 {
				return restriction.checkAny()
			}
		case "resources":
			if override.Resources != nil {
				return restriction.checkAny()
			}
		}

		return nil
	}

	switch root {
	case "volumes":
		return checkVolumes(override.Volumes, remaining, restriction)
	case "securityContext":
		return checkPodSecurityContext(override.SecurityContext, remaining, restriction)
	case "imagePullSecrets":
		return checkImagePullSecrets(override.ImagePullSecrets, remaining, restriction)
	case "overhead":
		return checkResourceList(override.Overhead, remaining, restriction)
	case "resourceClaims":
		return checkResourceClaims(override.ResourceClaims, remaining, restriction)
	case "resources":
		return checkResources(override.Resources, remaining, restriction)
	}

	return nil
}

func checkVolumes(volumes []corev1.Volume, field string, restriction *FieldRestriction) error {
	if len(volumes) == 0 {
		return nil
	}

	for _, volume := range volumes {
		if err := checkVolume(volume, field, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkVolume(volume corev1.Volume, field string, restriction *FieldRestriction) error {
	switch field {
	case "name":
		return restriction.checkString(&volume.Name)
	default:
		volType := volumeSourceType(&volume)
		if volType != "" && volType == field {
			return restriction.checkAny()
		}
	}

	return nil
}

func checkPodSecurityContext(sc *corev1.PodSecurityContext, field string, restriction *FieldRestriction) error {
	if sc == nil {
		return nil
	}

	root, remaining, _ := strings.Cut(field, ".")

	switch root {
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
	case "supplementalGroups":
		return checkSupplementalGroups(sc.SupplementalGroups, remaining, restriction)
	case "supplementalGroupsPolicy":
		return restriction.checkString((*string)(sc.SupplementalGroupsPolicy))
	case "fsGroup":
		return restriction.checkInt64(sc.FSGroup)
	case "sysctls":
		return checkSysctls(sc.Sysctls, remaining, restriction)
	case "fsGroupChangePolicy":
		return restriction.checkString((*string)(sc.FSGroupChangePolicy))
	case "seccompProfile":
		if sc.SeccompProfile != nil {
			return restriction.checkAny()
		}
	case "appArmorProfile":
		if sc.AppArmorProfile != nil {
			return restriction.checkAny()
		}
	case "seLinuxChangePolicy":
		return restriction.checkString((*string)(sc.SELinuxChangePolicy))
	}

	return nil
}

func checkSysctls(sysctls []corev1.Sysctl, field string, restriction *FieldRestriction) error {
	if len(sysctls) == 0 {
		return nil
	}

	if err := restriction.checkAny(); err != nil {
		return err
	}

	for _, sysctl := range sysctls {
		switch field {
		case "name":
			if err := restriction.checkString(&sysctl.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkSupplementalGroups(supplementalGroups []int64, field string, restriction *FieldRestriction) error {
	if len(supplementalGroups) == 0 {
		return nil
	}

	if err := restriction.checkAny(); err != nil {
		return err
	}

	for _, supplementalGroup := range supplementalGroups {
		if field == "" {
			if err := restriction.checkInt64(&supplementalGroup); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkResourceClaims(claims []corev1.PodResourceClaim, field string, restriction *FieldRestriction) error {
	if len(claims) == 0 {
		return nil
	}

	for _, claim := range claims {
		if err := checkResourceClaim(claim, field, restriction); err != nil {
			return err
		}
	}

	return nil
}

func checkResourceClaim(claim corev1.PodResourceClaim, field string, restriction *FieldRestriction) error {
	switch field {
	case "name":
		return restriction.checkString(&claim.Name)
	case "resourceClaimName":
		return restriction.checkString(claim.ResourceClaimName)
	case "resourceClaimTemplateName":
		return restriction.checkString(claim.ResourceClaimTemplateName)
	}

	return nil
}

func checkImagePullSecrets(secrets []corev1.LocalObjectReference, field string, restriction *FieldRestriction) error {
	if len(secrets) == 0 {
		return nil
	}

	for _, secret := range secrets {
		switch field {
		case "name":
			if err := restriction.checkString(&secret.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func volumeSourceType(vol *corev1.Volume) string {
	src := vol.VolumeSource
	switch {
	case src.HostPath != nil:
		return "hostPath"
	case src.EmptyDir != nil:
		return "emptyDir"
	case src.Secret != nil:
		return "secret"
	case src.ConfigMap != nil:
		return "configMap"
	case src.PersistentVolumeClaim != nil:
		return "persistentVolumeClaim"
	case src.Projected != nil:
		return "projected"
	case src.DownwardAPI != nil:
		return "downwardAPI"
	case src.CSI != nil:
		return "csi"
	case src.Ephemeral != nil:
		return "ephemeral"
	case src.NFS != nil:
		return "nfs"
	case src.FC != nil:
		return "fc"
	case src.Image != nil:
		return "image"
	case src.ISCSI != nil:
		return "iscsi"
	case src.GCEPersistentDisk != nil:
		return "gcePersistentDisk"
	case src.AWSElasticBlockStore != nil:
		return "awsElasticBlockStore"
	case src.GitRepo != nil:
		return "gitRepo"
	case src.Glusterfs != nil:
		return "glusterfs"
	case src.RBD != nil:
		return "rbd"
	case src.FlexVolume != nil:
		return "flexVolume"
	case src.Cinder != nil:
		return "cinder"
	case src.CephFS != nil:
		return "cephfs"
	case src.Flocker != nil:
		return "flocker"
	case src.AzureFile != nil:
		return "azureFile"
	case src.VsphereVolume != nil:
		return "vsphereVolume"
	case src.Quobyte != nil:
		return "quobyte"
	case src.AzureDisk != nil:
		return "azureDisk"
	case src.PhotonPersistentDisk != nil:
		return "photonPersistentDisk"
	case src.PortworxVolume != nil:
		return "portworxVolume"
	case src.ScaleIO != nil:
		return "scaleIO"
	case src.StorageOS != nil:
		return "storageos"
	default:
		return ""
	}
}
