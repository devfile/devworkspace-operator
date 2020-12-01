# Dev Workspace Operator

Dev Workspace operator repository that contains the controller for the DevWorkspace Custom Resource. The Kubernetes API of the DevWorkspace is defined in the https://github.com/devfile/api repository.

## DevWorkspace CR

### Annotations

You can add these Kubernetes annotations to specific DevWorkspace CR to customize their behavior.

|Name|Value|
|----|----|
|[controller.devfile.io/restricted-access](#restricted-access)|true or false|

#### Restricted Access

Using `restricted-access` is possible to define that DevWorkspace needs additional(to RBAC) authorization that guarantee that the only DevWorkspace CR creators has access to the containers terminals and secure server.
May be needed when personal information(like access tokens, ssh keys) is stored in the containers.

Since it's powered by webhooks, DevWorkspaces with such annotations will fails to start when webhooks are disabled on Operator level. 

Example:
```yaml
metadata:
  annotations:
    controller.devfile.io/restricted-access: true
```

## Running the controller in a cluster

The controller can be deployed to a cluster provided you are logged in with cluster-admin credentials:

```bash
export IMG=quay.io/devfile/devworkspace-controller:next
make install
```

By default, controller will expose workspace servers without any authentication; this is not advisable for public clusters, as any user could access the created workspace via URL.

In case of OpenShift, you're able to configure DevWorkspace CR to secure your servers with the following piece of configuration:

```yaml
spec:
  routingClass: openshift-oauth
```

See below for all environment variables used in the makefile.

> Note: The operator requires internet access from containers to work. By default, `crc setup` may not provision this, so it's necessary to configure DNS for Docker:
> ```
> # /etc/docker/daemon.json
> {
>   "dns": ["192.168.0.1"]
> }
> ```

## Development

The repository contains a Makefile; building and deploying can be configured via the environment variables

|variable|purpose|default value|
|---|---|---|
| `IMG` | Image used for controller | `quay.io/devfile/devworkspace-controller:next` |
| `NAMESPACE` | Namespace to use for deploying controller | `devworkspace-controller` |
| `ROUTING_SUFFIX` | Cluster routing suffix (e.g. `$(minikube ip).nip.io`, `apps-crc.testing`). Required for Kubernetes | `192.168.99.100.nip.io` |
| `PULL_POLICY` | Image pull policy for controller | `Always` |
| `WEBHOOK_ENABLED` | Whether webhooks should be enabled in the deployment | `false` |
| `REGISTRY_ENABLED` | Whether the plugin registry should be deployed | `true` |
| `DEVWORKSPACE_API_VERSION` | Branch or tag of the github.com/devfile/api to depend on | `v1alpha1` | 

Some of the rules supported by the makefile:

|rule|purpose|
|---|---|
| docker | build and push docker image |
| install | install controller to cluster |
| restart | restart cluster controller deployment |
| install_crds | update CRDs on cluster |
| uninstall | delete controller namespace `devworkspace-controller` and remove CRDs from cluster |
| help | print all rules and variables |

To see all rules supported by the makefile, run `make help`

### Test run controller
1. Take a look samples workspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/workspace_java_mysql.yaml -n <namespace>`
3. As soon as workspace is started you're able to get IDE url by executing `kubectl get devworkspace -n <namespace>`

### Run controller locally
It's possible to run an instance of the controller locally while communicating with a cluster. However, this requires webhooks to be disabled, as the webhooks need to be able to access the service created by an in-cluster deployment

```bash
make install
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}"
make debug
```

When running locally, only a single namespace is watched; as a result, all workspaces have to be deployed to `${NAMESPACE}`

### Run controller locally and debug
Debugging the controller depends on `delve` being installed (`go get -u github.com/go-delve/delve/cmd/dlv`). Note that at the time of writing, executing `go get` in this repo's directory will update go.mod; these changes should be dropped before committing.

```bash
make install
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}"
make debug
```

### Controller configuration

Controller behavior can be configured with data from the `devworkspace-controller` config map in the same namespace where controller lives.

For all available configuration properties and their default values, see [pkg/config](https://github.com/devfile/devworkspace-operator/tree/master/pkg/config)

### Remove controller from your K8s/OS Cluster
To uninstall the controller and associated CRDs, use the Makefile uninstall rule:
```bash
make uninstall
```
This will delete all custom resource definitions created for the controller, as well as the `devworkspace-controller` namespace.

### CI

#### GitHub actions

- [Next Dockerimage](https://github.com/devfile/devworkspace-operator/blob/master/.github/workflows/dockerimage-next.yml) action build master and push it to [quay.io/devfile/devworkspace-controller:next](https://quay.io/repository/devfile/devworkspace-controller?tag=latest&tab=tags)
