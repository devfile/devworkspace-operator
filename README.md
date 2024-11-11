<p align="center"><img alt="DevWorkspace operator" src="./img/logo.png" width="150px" /></p>

# DevWorkspace Operator

[![codecov](https://codecov.io/gh/devfile/devworkspace-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/devfile/devworkspace-operator)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/8258/badge)](https://www.bestpractices.dev/projects/8258)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/devfile/devworkspace-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/devfile/devworkspace-operator)

DevWorkspace operator repository that contains the controller for the DevWorkspace Custom Resource. The Kubernetes API of the DevWorkspace is defined in the https://github.com/devfile/api repository.

## What is the DevWorkspace Operator?

A [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) to run **fast**, **repeatable**
and **scalable** Cloud Development Environments.

[Install it](#deploying-devworkspace-operator) and apply a DevWorkspace to create a Cloud Development Environment:<br/>
![dw apply demo](img/apply-demo.gif)

Get the Cloud Developent Environment URI:<br/>
![dw get demo](img/get-demo.gif)

Open the IDE:<br/>
| Visual Studio Code  | JetBrains IntelliJ |
| ------------- | ------------- |
| ![vscode](img/vscode.png) | ![intellij](img/intellij.png) |

## Example

Here is a sample `DevWorkspace` to provision a Cloud Development Environment for the project
[github.com/l0rd/outyet](https://github.com/l0rd/outyet) with Visual Studio Code as the editor and
`quay.io/devfile/universal-developer-image:ubi8-latest` as the development tooling container image.<br/>

![devworkspace](img/devworkspace.png)

#### DevWorkspace Template

The Template section of a `DevWorkspace` is actually [a Devfile](https://devfile.io/docs/2.3.0/what-is-a-devfile): the
`spec.template` schema matches the [Devfile schema](https://devfile.io/docs/2.3.0/devfile-schema). :warning: A few 
`Devfile` APIs are
[not supported yet](https://github.com/devfile/devworkspace-operator/blob/main/docs/unsupported-devfile-api.adoc).

#### DevWorkspace Contributions

Contributions are extra `Templates` that are added on top of the main `DevWorkspaceTemplate`. Contributions are used to
inject editors such as Visual Studio Code and JetBrains. Contributions are defined as Devfile or DevWorkspace Templates.
Examples are the
[Visual Studio Code devfile](https://eclipse-che.github.io/che-plugin-registry/main/v3/plugins/che-incubator/che-code/latest/devfile.yaml)
and the
[JetBrains IntelliJ devfile](https://eclipse-che.github.io/che-plugin-registry/main/v3/plugins/che-incubator/che-idea/latest/devfile.yaml).

#### Additional configuration

DevWorkspaces can be further configured through DevWorkspace `attributes`, `labels` and `annotations`. For a list of all
options available, see [additional documentation](docs/additional-configuration.adoc).

## Deploying DevWorkspace Operator

### Prerequisites
- go 1.16 or later
- git
- sed
- jq
- yq (python-yq from https://github.com/kislyuk/yq#installation, other distributions may not work)
- skopeo (if building the OLM catalogsource)
- podman or docker

Note: kustomize `v4.0.5` is required for most tasks. It is downloaded automatically to the `.kustomize` folder in this repo when required. This downloaded version is used regardless of whether or not kustomize is already installed on the system.

### Running the controller in a cluster

#### With yaml resources

When installing on Kubernetes clusters, the DevWorkspace Operator requires the [cert-manager](https://cert-manager.io) operator in order to properly serve webhooks. To install the latest version of cert-manager in a cluster, the Makefile rule `install_cert_manager` can be used. The minimum version of cert-manager is `v1.0.4`.

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

#### With Operator Lifecycle Manager (OLM)

DevWorkspace Operator has bundle and index images which enable installation via OLM. To enable installing the DevWorkspace Operator through OLM, it may be necessary to create a CatalogSource in the cluster for this index:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: devworkspace-operator-catalog
  namespace: openshift-marketplace # Namespace for catalogsource, not operator itself
spec:
  sourceType: grpc
  image: quay.io/devfile/devworkspace-operator-index:next
  publisher: Red Hat
  displayName: DevWorkspace Operator Catalog
  updateStrategy:
    registryPoll:
      interval: 5m
```

Two index images are available for installing the DevWorkspace Operator:
* `quay.io/devfile/devworkspace-operator-index:release` - multi-version catalog with all DevWorkspace Operator releases
* `quay.io/devfile/devworkspace-operator-index:next` - single-version catalog that will deploy the latest commit in the `main` branch

Both index images allow automatic updates (to either the latest release or latest commit in main).

After OLM finishes processing the created CatalogSource, DWO should appear on the Operators page in the OpenShift Console.

In order to build a custom bundle, the following environment variables should be set:
| variable | purpose | default value |
|---|---|---|
| `DWO_BUNDLE_IMG` | Image used for Operator bundle image | `quay.io/devfile/devworkspace-operator-bundle:next` |
| `DWO_INDEX_IMG` | Image used for Operator index image | `quay.io/devfile/devworkspace-operator-index:next` |
| `DEFAULT_DWO_IMG` | Image used for controller when generating defaults | `quay.io/devfile/devworkspace-controller:next` |

To build the index image and register its catalogsource to the cluster, run
```
make generate_olm_bundle_yaml build_bundle_and_index register_catalogsource
```

Note that setting `DEFAULT_DWO_IMG` while generating sources will result in local changes to the repo which should be `git restored` before committing. This can also be done by unsetting the `DEFAULT_DWO_IMG` env var and re-running `make generate_olm_bundle_yaml`

## Development

The repository contains a Makefile; building and deploying can be configured via the environment variables

|variable|purpose|default value|
|---|---|---|
| `DWO_IMG` | Image used for controller | `quay.io/devfile/devworkspace-controller:next` |
| `DEFAULT_DWO_IMG` | Image used for controller when generating default deployment templates. Can be used to override the controller image in the OLM bundle | `quay.io/devfile/devworkspace-controller:next` |
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
| install_cert_manager | installs the cert-manager to the cluster (only required for Kubernetes) |
| uninstall | delete controller namespace `devworkspace-controller` and remove CRDs from cluster |
| help | print all rules and variables |

To see all rules supported by the makefile, run `make help`

### Test run controller
1. Take a look samples devworkspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/code-latest.yaml -n <namespace>`
3. As soon as devworkspace is started you're able to get IDE url by executing `kubectl get devworkspace -n <namespace>`

### Run controller locally
```bash
export NAMESPACE="devworkspace-controller"
make install
# Wait for webhook server to start
kubectl rollout status deployment devworkspace-controller-manager -n $NAMESPACE --timeout 90s
kubectl rollout status deployment devworkspace-webhook-server -n $NAMESPACE --timeout 90s
# Scale on-cluster deployment to zero to avoid conflict with locally-running instance
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}" -n $NAMESPACE
make run
```

### Run controller locally and debug
Debugging the controller depends on [delve](https://github.com/go-delve/delve) being installed (`go install github.com/go-delve/delve/cmd/dlv@latest`). Note that `$GOPATH/bin` or `$GOBIN` must be added to `$PATH` in order for `make debug` to run correctly.

```bash
make install
# Wait for webhook server to start
kubectl rollout status deployment devworkspace-controller-manager -n $NAMESPACE --timeout 90s
kubectl rollout status deployment devworkspace-webhook-server -n $NAMESPACE --timeout 90s
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}"
# Scale on-cluster deployment to zero to avoid conflict with locally-running instance
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

### Updating devfile API

[devfile API](https://github.com/devfile/api) is the Kube-native API for cloud development workspaces specification and the core dependency of the devworkspace-operator that should be regularly updated to the latest version. In order to do the update:

1. update `DEVWORKSPACE_API_VERSION` variable in the `Makefile` and `build/scripts/generate_deployment.sh`. The variable should correspond to the commit SHA from the [devfile API](https://github.com/devfile/api) repository
2. run the following scripts and the open pull request

```bash
make update_devworkspace_api update_devworkspace_crds # first commit
make generate_all # second commit
```
Example of the devfile API update [PR](https://github.com/devfile/devworkspace-operator/pull/797)

### Remove controller from your K8s/OS Cluster
To uninstall the controller and associated CRDs, use the Makefile uninstall rule:
```bash
make uninstall
```
This will delete all custom resource definitions created for the controller, as well as the `devworkspace-controller` namespace.

## CI

#### GitHub actions

- [Next Dockerimage](https://github.com/devfile/devworkspace-operator/blob/main/.github/workflows/dockerimage-next.yml) action builds main branch and pushes it to [quay.io/devfile/devworkspace-controller:next](https://quay.io/repository/devfile/devworkspace-controller?tag=latest&tab=tags)

- [Code Coverage Report](./.github/workflows/code-coverage.yml) action creates a code coverage report using [codecov.io](https://about.codecov.io/).

## Contributing

For information on contributing to this project please see [CONTRIBUTING.md](CONTRIBUTING.md).
