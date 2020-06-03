package tests

import (
	"fmt"
	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/workspaces"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)
var client workspaces.CodeReady

var _ = ginkgo.Describe("[Create Cloud Shell Workspace]", func() {
	ginkgo.It("Add cloud shell to cluster", func() {
		label := "che.workspace_name=cloud-shell"
		workspace := workspaces.NewWorkspaceClient()
		_ = workspace.AddCloudShellSample()
		deploy , err := client.PodDeployWaitUtil(label)
		if !deploy {
			fmt.Println("Cloud Shell not deployed")
		}
		Expect(err).NotTo(HaveOccurred())
	})
})
