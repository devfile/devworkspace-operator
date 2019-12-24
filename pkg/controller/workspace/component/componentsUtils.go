package component

import (
	"strings"
	"github.com/eclipse/che-plugin-broker/model"
	workspaceApi "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sModelUtils "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/modelutils/k8s"

	. "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace/model"
)

func createVolumeMounts(workspaceProps WorkspaceProperties, mountSources *bool, devfileVolumes []workspaceApi.Volume, pluginVolumes []model.Volume) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	volumeName := "claim-che-workspace"

	for _, volDef := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.ContainerPath,
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + "/" + volDef.Name + "/",
		})
	}
	for _, volDef := range pluginVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.MountPath,
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + "/" + volDef.Name + "/",
		})
	}

	if mountSources != nil && *mountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: "/projects",
			Name:      volumeName,
			SubPath:   workspaceProps.WorkspaceId + "/projects/",
		})
	}

	return volumeMounts
}

func createK8sServicesForMachines(wkspProps WorkspaceProperties, machineName string, exposedPorts []int) []corev1.Service {
	services := []corev1.Service {}
	servicePorts := k8sModelUtils.BuildServicePorts(exposedPorts, corev1.ProtocolTCP)
	serviceName := machineServiceName(wkspProps, machineName)
	if len(servicePorts) > 0 {
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: wkspProps.Namespace,
				Annotations: map[string]string{
					"org.eclipse.che.machine.name":   machineName,
				},
				Labels: map[string]string{
					"che.workspace_id": wkspProps.WorkspaceId,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"che.original_name": CheOriginalName,
					"che.workspace_id":  wkspProps.WorkspaceId,
				},
				Type:  corev1.ServiceTypeClusterIP,
				Ports: servicePorts,
			},
		}
		services = append(services, service)
	}
	return services
}

func interpolate(someString string, wkspProps WorkspaceProperties) string {
	for _, envVar := range commonEnvironmentVariables(wkspProps) {
		someString = strings.ReplaceAll(someString, "${" + envVar.Name + "}", envVar.Value)
	}
	return someString
}