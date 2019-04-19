package apis

import (
	apis "github.com/che-incubator/che-workspace-crd-operator/pkg/apis/workspace/v1alpha1"
	controller "github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspace"
	routeV1 "github.com/openshift/api/route/v1"
	templateV1 "github.com/openshift/api/template/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		apis.SchemeBuilder.AddToScheme,
	)
	if isOS, err := controller.IsOpenShift(); isOS && err == nil {
		AddToSchemes = append(AddToSchemes,
			routeV1.AddToScheme,
			templateV1.AddToScheme,
		)
	}
}
