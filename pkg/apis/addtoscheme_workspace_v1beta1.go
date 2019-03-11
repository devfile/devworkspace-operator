// MY LICENSEdep

package apis

import (
	"github.com/che-incubator/che-workspace-crd-controller/pkg/apis/workspace/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v1beta1.SchemeBuilder.AddToScheme)
}
