[![Master Build Status](https://ci.centos.org/buildStatus/icon?subject=master&job=devtools-che-workspace-operator-build-master/)](https://ci.centos.org/view/Devtools/job/devtools-che-workspace-operator-build-master/)
[![Nightly Build Status](https://ci.centos.org/buildStatus/icon?subject=nightly&job=devtools-che-workspace-operator-nightly/)](https://ci.centos.org/view/Devtools/job/devtools-che-workspace-operator-nightly/)

# Che Workspace Operator

Che workspace operator repository that contains K8s API for Che workspace and controller for them.

## Running the controller in a cluster

The controller can be deployed to a cluster provided you are logged in with cluster-admin credentials:

```bash
export IMG=quay.io/che-incubator/che-workspace-controller:nightly
export TOOL=oc # Use 'export TOOL=kubectl' for kubernetes
make deploy
```

By default, controller will expose workspace servers without any authentication; this is not advisable for public clusters, as any user could access the created workspace via URL.

In case of OpenShift, you're able to configure controller to secure your workspaces server deploy with the following options:

```bash
export WEBHOOK_ENABLED=true
export DEFAULT_ROUTING=openshift-oauth
make deploy
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
| `IMG` | Image used for controller | `quay.io/che-incubator/che-workspace-controller:nightly` |
| `TOOL` | CLI tool for interfacing with the cluster: `kubectl` or `oc`; if `oc` is used, deployment is tailored to OpenShift, otherwise Kubernetes | `oc` |
| `ROUTING_SUFFIX` | Cluster routing suffix (e.g. `$(minikube ip).nip.io`, `apps-crc.testing`). Required for Kubernetes | `192.168.99.100.nip.io` |
| `PULL_POLICY` | Image pull policy for controller | `Always` |
| `WEBHOOK_ENABLED` | Whether webhooks should be enabled in the deployment | `false` |
| `DEFAULT_ROUTING` | Default routingClass to apply to workspaces that don't specify one | `basic` |
| `ADMIN_CTX` | Kubectx entry that should be used during work with cluster. The current will be used if omitted |-|
| `REGISTRY_ENABLED` | Whether the plugin registry should be deployed | `true` |

Some of the rules supported by the makefile:

|rule|purpose|
|---|---|
| docker | build and push docker image |
| webhook | generate certificates for webhooks and deploy to cluster; no-op if webhooks are disabled or running on OpenShift |
| deploy | deploy controller to cluster |
| restart | restart cluster controller deployment |
| rollout | rebuild and push docker image and restart cluster deployment |
| update_cfg | configures already deployed controller according to set env variables |
| update_crds | update custom resource definitions on cluster |
| uninstall | delete controller namespace `che-workspace-controller` and remove custom resource definitions from cluster |
| help | print all rules and variables |

To see all rules supported by the makefile, run `make help`

### Test run controller
1. Take a look samples workspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/workspace_java_mysql.yaml -n <namespace>`
3. As soon as workspace is started you're able to get IDE url by executing `kubectl get workspace -n <namespace>`

### Run controller locally
It's possible to run an instance of the controller locally while communicating with a cluster. However, this requires webhooks to be disabled, as the webhooks need to be able to access the service created by an in-cluster deployment

```bash
export NAMESPACE=che-workspace-controller
export TOOL=oc # Use 'export TOOL=kubectl' for kubernetes
export WEBHOOK_ENABLED=false
make local
operator-sdk up local --namespace ${NAMESPACE}
```

When running locally, only a single namespace is watched; as a result, all workspaces have to be deployed to `${NAMESPACE}`

### Run controller locally and debug
Debugging the controller depends on `delve` being installed (`go get -u github.com/go-delve/delve/cmd/dlv`). Note that at the time of writing, executing `go get` in this repo's directory will update go.mod; these changes should be dropped before committing.

```bash
export NAMESPACE=che-workspace-controller
export TOOL=oc # Use 'export TOOL=kubectl' for kubernetes
export WEBHOOK_ENABLED=false
make local
operator-sdk up local --namespace ${NAMESPACE} --enable-delve
```

### Controller configuration

Controller behavior can be configured with data from the `che-workspace-controller` config map in the same namespace where controller lives.

For all available configuration properties and their default values, see [pkg/config](https://github.com/devfile/devworkspace-operator/tree/master/pkg/config)

### Remove controller from your K8s/OS Cluster
To uninstall the controller and associated CRDs, use the Makefile uninstall rule:
```bash
make uninstall
```
This will delete all custom resource definitions created for the controller, as well as the `che-workspace-controller` namespace.

### CentOS CI
The following [CentOS CI jobs](https://ci.centos.org/) are associated with the repository:

- [`master`](https://ci.centos.org/job/devtools-che-workspace-operator-build-master/) - builds CentOS images on each commit to the [`master`](https://github.com/devfile/devworkspace-operator/tree/master) branch and pushes them to [quay.io/che-incubator/che-workspace-controller](https://quay.io/repository/che-incubator/che-workspace-controller).
- [`nightly`](https://ci.centos.org/job/devtools-che-workspace-operator-nightly/) - builds CentOS images and pushes them to [quay.io/che-incubator/che-workspace-controller](https://quay.io/repository/che-incubator/che-workspace-controller) on a daily basis from the [`master`](https://github.com/devfile/devworkspace-operator/tree/master) branch.
