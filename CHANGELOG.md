# DevWorkspace Operator Changelog

# v0.37.0
## Features
### Add hostUsers field to DWOC [#1493](https://github.com/devfile/devworkspace-operator/issues/1493)
The DevWorkspace pod's `spec.hostUsers` field can now be set in the DevWorkspaceOperatorConfig:
```
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
config:
  workspace:
    hostUsers: false
``` 
Setting the `spec.hostUsers` field to `false` is useful when leveraging user-namespaces for pods.

**Note:** This field is only respected when the UserNamespacesSupport feature is enabled in the cluster. If the feature is disabled, setting hostUsers: false may lead to an endless workspace start loop.

### Provide timeout for postStart events [#1496](https://github.com/devfile/devworkspace-operator/issues/1496)
A timeout can now be configured for postStart events to prevent workspace pods from being stuck in a terminating state:
```
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
config:
  workspace:
    postStartTimeout: 30 
```
By default, this timeout is disabled.

## Bug Fixes & Improvements
- Add E2E test to verify that DevWorkspace contents are persisted during restarts [#1467](https://github.com/devfile/devworkspace-operator/issues/1467)
- make run fails on m4 MacBook [#1494](https://github.com/devfile/devworkspace-operator/issues/1494)
- Upgrade Go toolchain to 1.24.6 [#1509](https://github.com/devfile/devworkspace-operator/issues/1509)

# v0.36.0
## Bug Fixes & Improvements
### Remove `kube-rbac-proxy` from the controller Deployment [#1437](https://github.com/devfile/devworkspace-operator/pull/1437)
The `kube-rbac-proxy` container is now removed from the `devworkspace-controller-manager` Deployment. Instead, the metrics endpoint is protected with the [WithAuthenticationAndAuthorization](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/metrics/filters#WithAuthenticationAndAuthorization) feature provided by the [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) project.

This allows setting only the controller container's resource constraints using the Subscription resource as defined [here](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/subscription-config.md#example-4). The resource constraints defined in the Subscription apply the constraints to all containers in the `devworkspace-controller-manager` Deployment. As a result, previously both the controller and kube-rbac-proxy container's constraints were updated, when in most cases only the controller container was the desired container to be updated.

### Execute preStart devfile events after the project-clone init-container has run [#1454](https://github.com/devfile/devworkspace-operator/issues/1454)
The `project-clone` init container is now the first init container for DevWorkspace Pods. This ensures that the project is already cloned when running pre-start events defined in the Devfile.

## Bug Fixes & Improvements
- Update golang version to 1.24 in go.mod [#1413](https://github.com/devfile/devworkspace-operator/pull/1413)
- Update cleanup job node affinity logic [#1455](https://github.com/devfile/devworkspace-operator/pull/1455)

# v0.35.1
## Bug Fixes & Improvements
- Reverted [#1269](https://github.com/devfile/devworkspace-operator/issues/1269) due to [#1453](https://github.com/devfile/devworkspace-operator/issues/1453)

# v0.35.0

## Features
### Make workspace PVC's Access mode configurable [#1019](https://github.com/devfile/devworkspace-operator/issues/1019)
It is now possible to configure the storage access mode of per-user and per-workspace PVCs from the global `DevWorkspaceOperatorConfig`. For example:
```
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  workspace:
    imagePullPolicy: Always
    storageAccessMode:
    - ReadWriteMany
```

## Bug Fixes & Improvements
- Some tests do not run locally (macOS) [#1387](https://github.com/devfile/devworkspace-operator/issues/1387)
- Common PVC cleanup job can be assigned to incorrect node in multi-node cluster [#1269](https://github.com/devfile/devworkspace-operator/issues/1269)
