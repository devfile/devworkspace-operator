package utils
import (
	"github.com/eclipse/che-plugin-broker/model"
)

func ExposedPortsToInts(exposedPorts []model.ExposedPort) []int {
	ports := []int {}
	for _, exposedPort := range exposedPorts {
		ports = append(ports, exposedPort.ExposedPort)
	}
	return ports
}