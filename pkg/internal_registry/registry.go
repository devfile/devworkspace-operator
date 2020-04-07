package registry

import (
	brokerModel "github.com/eclipse/che-plugin-broker/model"
)

func InternalRegistry() map[string]brokerModel.PluginMeta {

	return map[string]brokerModel.PluginMeta{
		"eclipse/cloud-shell/nightly": {
			APIVersion:  "v2",
			Publisher:   "eclipse",
			Description: "Cloud Shell provides an ability to use terminal widget like an editor.",
			DisplayName: "Cloud Shell Editor",
			ID:          "",
			Icon:        "https://www.eclipse.org/che/images/logo-eclipseche.svg",
			Name:        "cloud-shell",
			Spec: brokerModel.PluginMetaSpec{
				Endpoints: []brokerModel.Endpoint{
					{
						Name:       "cloud-shell",
						Public:     true,
						TargetPort: 4444,
						Attributes: map[string]string{
							"protocol":           "http",
							"type":               "ide",
							"discoverable":       "false",
							"secure":             "true",
							"cookiesAuthEnabled": "true",
						},
					},
				},
				Containers: []brokerModel.Container{
					{
						Name:  "che-machine-exec",
						Image: "quay.io/eclipse/che-machine-exec:nightly",
						Ports: []brokerModel.ExposedPort{
							{
								ExposedPort: 4444,
							},
						},
						Command: []string{
							"/go/bin/che-machine-exec",
							"--static",
							"/cloud-shell",
							"--url",
							"127.0.0.1:4444",
						},
					},
				},
			},
			Title:   "Cloud Shell Editor",
			Type:    "Che Editor",
			Version: "nightly",
		},
	}
}
