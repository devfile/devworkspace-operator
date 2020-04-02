# Che Workspace Operator

Che workspace operator repository that contains K8s API for Che workspace and controller for them.

## Development

### Build docker image

To build docker image run the following command in the project's root

```
docker build -t quay.io/che-incubator/che-workspace-controller:7.1.0 -f ./build/Dockerfile .
```
> Webhooks are not configured automatically in non openshift environments
### Run controller within Kubernetes cluster
```bash
# 1. Install CRDs
kubectl apply -f ./deploy/crds
# 2. Create che-workspace-controller which will hosts controller
kubectl create namespace che-workspace-controller
# 3. Deploy Plugin Registry
# [!MANUAL_ACTION]: Make sure that the right domain is set in `./deploy/controller_config.yaml` and `./deploy/registry/local/ingress.yaml`
kubectl apply -f ./deploy/registry/local
kubectl apply -f ./deploy/registry/local/k8s
# 4. Generate certificates for Webhook server by executing
./deploy/webhook-server-certs/deploy-webhook-server-certs.sh kubectl
# 5. Deploy Controller itself
# [OPTIONAL MANUAL ACTION] Modify ./deploy/k8s/controller.yaml and put your docker image and pull policy there.
kubectl apply -f ./deploy/k8s/controller.yaml
kubectl apply -f ./deploy
```

### Run controller within OpenShift cluster
> The operator requires internet access from containers to work. By default, `crc setup` may not provision this, so it's necessary to configure DNS for Docker:
> ```
> # /etc/docker/daemon.json
> {
>   "dns": ["192.168.0.1"]
> }
> ```

```bash
# 1. Install CRDs
oc apply -f ./deploy/crds
# 2. Create che-workspace-controller which will hosts controller
oc create namespace che-workspace-controller
# 3. Deploy Plugin Registry
oc apply -f ./deploy/registry/local
oc apply -f ./deploy/registry/local/os
PLUGIN_REGISTRY_HOST=$(oc get route che-plugin-registry -n che-workspace-controller -o jsonpath='{.spec.host}' || echo "")
# 4. Deploy Controller itself
# [OPTIONAL MANUAL ACTION] Modify ./deploy/os/controller.yaml and put your docker image and pull policy there.
oc apply -f ./deploy/os/controller.yaml
cat ./deploy/*.yaml | \
  sed "s|plugin.registry.url: .*|plugin.registry.url: http://${PLUGIN_REGISTRY_HOST}/v3|" | \
  oc apply -f -
```

### Run controller locally
According to your Cluster do 1-4 steps from [Kubernetes](#run-controller-within-kubernetes-cluster) or [OpenShift](#run-controller-within-openshift-cluster).

`operator-sdk up local --namespace <your watched namespace>`

### Test run controller
1. Take a look samples workspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/workspace_java_mysql.yaml -n <namespace>`
3. As soon as workspace is started you're able to get IDE url by executing `kubectl get workspace -n <namespace>`

### Run controller locally and debug
This depends on `delve` being installed (`go get -u github.com/go-delve/delve/cmd/dlv`). Note that at the time of writing, executing `go get` in this repo's directory will update go.mod; these changes should be dropped before committing.

Running the controller outside of the cluster depends on everything being in one namespace (e.g. `che-workspace-controller`).

1. Follow steps 1-5 for running the controller locally above
2. `operator-sdk up local --namespace <your namespace> --enable-delve`
3. Connect debugger to `127.0.0.1:2345` (see config in `.vscode/launch.json`)

### Remove controller from your K8s/OS Cluster
```sh
# Delete all workspaces
kubectl delete workspaces.workspace.che.eclipse.org --all-namespaces --all
# Remove contoller
kubectl delete namespace che-workspace-controller
# Remove CRDs
kubectl delete customresourcedefinitions.apiextensions.k8s.io workspaceroutings.workspace.che.eclipse.org
kubectl delete customresourcedefinitions.apiextensions.k8s.io workspaces.workspace.che.eclipse.org
```
