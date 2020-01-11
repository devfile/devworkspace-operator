# che-workspace-crd-operator

## Development

### Dependencies

Dependencies in the project are managed by Go Dep. After you added a dependency you need to run the following command to download dependencies to vendor repo and lock file and then commit changes:

```
dep ensure
```

Note that dep ensure doesn't automatically change Gopkg.toml which contains dependencies constraints. So, when a dependency is introduced or changed it should be reflected in Gopkg.toml.

### Build docker image

Since it uses Go Dep, you need exact operator-sdk 0.10.0 version where Go Modules is not a default vendor.

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