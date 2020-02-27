# Che Workspace Operator

Che workspace operator repository that contains K8s API for Che workspace and controller for them.

## Development

### Build docker image

To build docker image run the following command in the project's root

```
docker build -t quay.io/che-incubator/che-workspace-controller:7.1.0 -f ./build/Dockerfile .
```

### Run controller within K8s cluster
1. `kubectl apply -f ./deploy/crds`
2. `kubectl create namespace che-workspace-controller`
3. Make sure that the right domain is set in `./deploy/controller_config.yaml` and `./deploy/registry/local/ingress.yaml`
4. `kubectl apply -f ./deploy/registry/local`
5. [Optional] Modify ./deploy/controller.yaml and put your docker image and pull policy there.
6. `kubectl apply -f ./deploy`

### Run controller locally
1. `kubectl apply -f ./deploy/crds`
2. `kubectl create namespace che-workspace-controller`
3. Make sure that the right domain is set in `./deploy/controller_config.yaml` and `./deploy/registry/local/ingress.yaml`
4. `kubectl apply -f ./deploy/registry/local`
5. `kubectl apply -f ./deploy/controller_config.yaml`
6. `operator-sdk up local --namespace <your namespace>`

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
