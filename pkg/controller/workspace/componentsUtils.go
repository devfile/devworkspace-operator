package workspace

import (
	"github.com/eclipse/che-plugin-broker/model"
	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"
)

func createVolumeMounts(workspaceProps workspaceProperties, mountSources *bool, devfileVolumes []workspaceApi.Volume, pluginVolumes []model.Volume) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := "claim-che-workspace"

	for _, volDef := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.ContainerPath,
			Name:      volumeName,
			SubPath:   workspaceProps.workspaceId + "/" + volDef.Name + "/",
		})
	}
	for _, volDef := range pluginVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.MountPath,
			Name:      volumeName,
			SubPath:   workspaceProps.workspaceId + "/" + volDef.Name + "/",
		})
	}

	if mountSources != nil && *mountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: "/projects",
			Name:      volumeName,
			SubPath:   workspaceProps.workspaceId + "/projects/",
		})
	}

	return volumeMounts
}

func createK8sServicesForMachines(wkspProps workspaceProperties, machineName string, exposedPorts []int) []corev1.Service {
	services := []corev1.Service {}
	servicePorts := k8sModelUtils.BuildServicePorts(exposedPorts, corev1.ProtocolTCP)
	serviceName := machineServiceName(wkspProps, machineName)
	if len(servicePorts) > 0 {
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: wkspProps.namespace,
				Annotations: map[string]string{
					"org.eclipse.che.machine.name":   machineName,
				},
				Labels: map[string]string{
					"che.workspace_id": wkspProps.workspaceId,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"che.original_name": cheOriginalName,
					"che.workspace_id":  wkspProps.workspaceId,
				},
				Type:  corev1.ServiceTypeClusterIP,
				Ports: servicePorts,
			},
		}
		services = append(services, service)
	}
	return services
}