## Preparing static yaml files for deployment on OpenShift and Kubernetes

The Dockerfile in this directory can be used to automatically build static yaml files that can be applied to a cluster to deploy the DevWorkspace Operator without the need for kustomize.

### Building the image
```bash
docker build -t <template-image> -f yaml-builder.Dockerfile .
```
The docker build is self-contained; it clones the repo and builds directly from that codebase. Supported args are the same as those used in the Makefile, i.e.

| Arg | Purpose | Default |
| --- | ------- | ------- |
| DEVWORKSPACE_BRANCH | Branch of this repo to clone & check out | master
| NAMESPACE | Namespace used for deployment | devworkspace-controller
| IMG | DevWorkspace Operator image to be deployed | quay.io/devfile/devworkspace-controller:next
| PULL_POLICY | Sidecar PullPolicy used for workspaces | Always
| DEFAULT_ROUTING | Default routingClass to use for DevWorkspaces that do not define one | basic
| DEVWORKSPACE_API_VERSION | Commit hash of devfile/api dependency for DevWorkspace and DevWorkspaceTemplate CRDs | aeda60d4361911da85103f224644bfa792498499

### Using the image
The container image above creates a tarball of the relevant deployment files, and can be extracted from the container:
```bash
docker create --name builder <template-image>
docker cp builder:/devworkspace_operator_templates.tar.gz ./devworkspace_operator_templates.tar.gz
docker rm builder
tar -xzf devworkspace_operator_templates.tar.gz
```

This will extract the files to the `deploy` directory, with subdirectories for the OpenShift and Kubernetes deployments of the operator (on OpenShift, the service-ca operator is used to provide certificates where necessary; on Kubernetes the deployment depends on the cert-manager operator and includes a Certificate object).

Within each platform-dependent directory, the (large) file `combined.yaml` is a single file that can be applied to deploy the operator, and `objects/` contains each object in `combined.yaml` named according to `<resource-name>.<k8s-kind>.yaml`

As the yaml generation happens statically, the configmap leaves the `devworkspace.routing.cluster_host_suffix` property unset; on Kubernetes a value must be provided here to correctly generate ingresses.