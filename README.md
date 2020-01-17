# che-workspace-crd-operator

## Development

### Build docker image

Since it uses Go Modules, you need operator-sdk 0.12.0+ version where Go Modules is a default vendor.

To build docker image run the following command in the project's root

```
operator-sdk build quay.io/che-incubator/che-workspace-crd-controller:7.1.0
```

### Run controller within K8s cluster
1. `kubectl apply -f ./deploy/crds`
2. `kubectl create namespace devworkspace-controller`
3. Make sure that the right domain is set in `./deploy/controller_config.yaml` and `./deploy/registry/local/ingress.yaml`
4. `kubectl apply -f ./deploy/registry/local`
5. [Optional] Modify ./deploy/operator.yaml and put your docker image and pull policy there.
6. `kubectl apply -f ./deploy`

### Run controller locally
1. `kubectl apply -f ./deploy/crds`
2. Make sure right ingress domain is set in `./deploy/registry/local/ingress.yaml` and execute `kubectl apply -f ./deploy/registry/local`
3. `operator-sdk up local --namespace <your namespace>`

### Test run controller

1. Take a look samples workspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/workspace_java_mysql.yaml -n <namespace>`
3. As soon as workspace is started you're able to get IDE url by executing `kubectl get workspace -n <namespace>`