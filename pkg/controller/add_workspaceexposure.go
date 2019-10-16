package controller

import (
	"github.com/che-incubator/che-workspace-crd-operator/pkg/controller/workspaceexposure"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, workspaceexposure.Add)
}
