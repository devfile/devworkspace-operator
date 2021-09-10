# Dev Workspace Operator

Dev Workspace operator repository that contains the controller for the DevWorkspace Custom Resource. The Kubernetes API of the DevWorkspace is defined in the https://github.com/devfile/api repository.

## DevWorkspace CR

### Annotations

You can add these Kubernetes annotations to specific DevWorkspace CR to customize their behavior.

|Name|Value|
|----|----|
|[controller.devfile.io/restricted-access](#restricted-access)|true or false|

#### Restricted Access

The `controller.devfile.io/restricted-access` specifies that a DevWorkspace needs additional access control (in addition to RBAC). When a DevWorkspace is created with the `controller.devfile.io/restricted-access` annotation set to `true`, the webhook server will guarantee
- Only the DevWorkspace Operator ServiceAccount or DevWorkspace creator can modify important fields in the devworksapce
- Only the DevWorkspace creator can create `pods/exec` into devworkspace-related containers.

This annotation should be used when a DevWorkspace is expected to contain sensitive information that should be protect above the protection provided by standard RBAC rules (e.g. if the DevWorkspace will store the user's OpenShift token in-memory).

Example:
```yaml
metadata:
  annotations:
    controller.devfile.io/restricted-access: true
```
## Prerequisites
- go
- git
- sed
- jq
- yq (python-yq from https://github.com/kislyuk/yq#installation, other distributions may not work)
- skopeo (if building the OLM catalogsource)
- docker

Note: kustomize `v4.0.5` is required for most tasks. It is downloaded automatically to the `.kustomize` folder in this repo when required. This downloaded version is used regardless of whether or not kustomize is already installed on the system.

## Running the controller in a cluster

### With yaml resources

When deployed to Kubernetes, the controller requires [cert-manager](https://cert-manager.io) running in the cluster.
You can install it using `make install_cert_manager` if you don't run it already.
The minimum version of cert-manager is `v1.0.4`.

The controller can be deployed to a cluster provided you are logged in with cluster-admin credentials:

```bash
export DWO_IMG=quay.io/devfile/devworkspace-controller:next
make install
```

By default, the controller will expose workspace servers without any authentication; this is not advisable for public clusters, as any user could access the created workspace via URL.

See below for all environment variables used in the makefile.

> Note: The operator requires internet access from containers to work. By default, `crc setup` may not provision this, so it's necessary to configure DNS for Docker:
> ```
> # /etc/docker/daemon.json
> {
>   "dns": ["192.168.0.1"]
> }
> ```

### With OLM

DevWorkspace Operator has bundle and index images which allow to install it with OLM.
You need to register custom catalog source to make it available on your cluster with help:
```
DWO_INDEX_IMG=quay.io/devfile/devworkspace-operator-index:next
make register_catalogsource
```
After OLM processed created catalog source, DWO should appear on Operators page of OpenShift Console.

## Development

The repository contains a Makefile; building and deploying can be configured via the environment variables

|variable|purpose|default value|
|---|---|---|
| `DWO_IMG` | Image used for controller | `quay.io/devfile/devworkspace-controller:next` |
| `NAMESPACE` | Namespace to use for deploying controller | `devworkspace-controller` |
| `ROUTING_SUFFIX` | Cluster routing suffix (e.g. `$(minikube ip).nip.io`, `apps-crc.testing`). Required for Kubernetes | `192.168.99.100.nip.io` |
| `PULL_POLICY` | Image pull policy for controller | `Always` |
| `DEVWORKSPACE_API_VERSION` | Branch or tag of the github.com/devfile/api to depend on | `v1alpha1` |

Some of the rules supported by the makefile:

|rule|purpose|
|---|---|
| docker | build and push docker image |
| install | install controller to cluster |
| restart | restart cluster controller deployment |
| install_crds | update CRDs on cluster |
| install_cert_manager | installs the cert-manager to the cluster (only required for Kubernetes) |
| uninstall | delete controller namespace `devworkspace-controller` and remove CRDs from cluster |
| help | print all rules and variables |

To see all rules supported by the makefile, run `make help`

### Test run controller
1. Take a look samples devworkspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/flattened_theia-next.yaml -n <namespace>`
3. As soon as devworkspace is started you're able to get IDE url by executing `kubectl get devworkspace -n <namespace>`

### Run controller locally
```bash
export NAMESPACE="devworkspace-controller"
make install
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}" -n $NAMESPACE
make run
```

When running locally, only a single namespace is watched; as a result, all devworkspaces have to be deployed to `${NAMESPACE}`

### Run controller locally and debug
Debugging the controller depends on `delve` being installed (`go get -u github.com/go-delve/delve/cmd/dlv`). Note that at the time of writing, executing `go get` in this repo's directory will update go.mod; these changes should be dropped before committing.

```bash
make install
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}"
make debug
```

### Run webhook server locally and debug
Debugging the webhook server depends on `telepresence` being installed (`https://www.telepresence.io/docs/latest/install/`). Teleprescence works by redirecting traffic going from the webhook-server in the cluster to the local webhook-server you will be running on your computer.

```bash
make debug-webhook-server
```

when you are done debugging you have to manually uninstall the telepresence agent

```bash
make disconnect-debug-webhook-server
```

### Controller configuration

Controller behavior can be configured with data from the `devworkspace-controller` config map in the same namespace where controller lives.

For all available configuration properties and their default values, see [pkg/config](https://github.com/devfile/devworkspace-operator/tree/main/pkg/config)

### Remove controller from your K8s/OS Cluster
To uninstall the controller and associated CRDs, use the Makefile uninstall rule:
```bash
make uninstall
```
This will delete all custom resource definitions created for the controller, as well as the `devworkspace-controller` namespace.

### CI

#### GitHub actions

- [Next Dockerimage](https://github.com/devfile/devworkspace-operator/blob/main/.github/workflows/dockerimage-next.yml) action builds main branch and pushes it to [quay.io/devfile/devworkspace-controller:next](https://quay.io/repository/devfile/devworkspace-controller?tag=latest&tab=tags)
