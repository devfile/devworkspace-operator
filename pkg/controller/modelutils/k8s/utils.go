package utils
import (
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	corev1 "k8s.io/api/core/v1"
)

func BuildContainerPorts(exposedPorts []int, protocol corev1.Protocol) []corev1.ContainerPort {
	containerPorts := []corev1.ContainerPort {}
	for _, exposedPort := range exposedPorts {
		containerPorts = append(containerPorts, corev1.ContainerPort {
			ContainerPort: int32(exposedPort),
			Protocol: protocol,
		})
	}
	return containerPorts
}

func ServicePortName(port int) string {
	return "srv-" + strconv.FormatInt(int64(port), 10)
}

func BuildServicePorts(exposedPorts []int, protocol corev1.Protocol) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort
	for _, port := range exposedPorts {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       ServicePortName(port),
			Protocol:   protocol,
			Port:       int32(port),
			TargetPort: intstr.FromInt(port),
		})
	}
return servicePorts
}