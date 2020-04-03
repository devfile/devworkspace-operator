# Che Workspace Operator

Che workspace operator repository that contains K8s API for Che workspace and controller for them.

## Running the controller in a cluster

The controller can be deployed to a cluster provided you are logged in with cluster-admin credentials:

```bash
export IMG=quay.io/che-incubator/che-workspace-controller:nightly
export TOOL=oc # Use 'export TOOL=kubectl' for kubernetes
export WEBHOOK_ENABLED=false
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
| `IMG` | Image used for controller | `quay.io/che-incubator/che-workspace-controller:7.1.0` |
| `TOOL` | CLI tool for interfacing with the cluster: `kubectl` or `oc`; if `oc` is used, deployment is tailored to OpenShift, otherwise Kubernetes | `oc` |
| `CLUSTER_IP` | For Kubernetes only, the ip address of the cluster (`minikube ip`) | `192.168.99.100` |
| `PULL_POLICY` | Image pull policy for controller | `Always` |
| `WEBHOOK_ENABLED` | Whether webhooks should be enabled in the deployment | `false` |
| `DEFAULT_ROUTING` | Default routingClass to apply to workspaces that don't specify one | `basic` |

The makefile supports the following rules:

|rule|purpose|
|---|---|
| docker | build and push docker container |
| webhook | set up certificates/secrets for webhooks support; no-op if webhooks disabled |
| deploy | deploy controller |
| restart | rollout the controller deployment in cluster |
| rollout | build and push docker image; rollout changes to controller deployment | 
| update | reapply crds and configmap |
| uninstall | delete controller namespace `che-workspace-controller` and remove custom resource definitions from cluster |
| local | set up cluster to support controller, but do not deploy it; intended for use with `operator-sdk up local` |

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

### Remove controller from your K8s/OS Cluster
To uninstall the controller and associated CRDs, use the Makefile uninstall rule:
```bash
make uninstall
```
This will delete all custom resource definitions created for the controller, as well as the `che-workspace-controller` namespace.
