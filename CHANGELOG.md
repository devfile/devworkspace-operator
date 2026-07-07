# DevWorkspace Operator Changelog

# v0.42.0

## Features

### Configurable field-level restrictions for container and pod overrides [#1653](https://github.com/devfile/devworkspace-operator/pull/1653)

Cluster administrators can now define deny rules in `DevWorkspaceOperatorConfig` to block specific fields or field values from being set via the `container-overrides` and `pod-overrides` DevWorkspace attributes. Restrictions are specified using the new `config.overrides.restrictedContainerOverrideFields` and `config.overrides.restrictedPodOverrideFields` fields.

On Kubernetes, a set of security-sensitive fields is denied out of the box, including privileged containers, running as root, host networking, and `hostPath` volumes—matching the restrictions that OpenShift enforces natively via SCCs. These defaults can be adjusted from the global `DevWorkspaceOperatorConfig` object.

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  overrides:
    restrictedContainerOverrideFields:
      - securityContext.privileged
      - securityContext.runAsUser
    restrictedPodOverrideFields:
      - hostNetwork
```

See [docs/dwo-configuration.md](docs/dwo-configuration.md#restricting-override-fields) for configuration details.

## Bug Fixes & Improvements

- Fix OpenShift registry image-puller RoleBinding accumulating deleted workspace ServiceAccounts, causing etcd rejections in high-churn namespaces [#1640](https://github.com/devfile/devworkspace-operator/issues/1640)
- Fix project cloning failing when a nested `clonePath` is used and intermediate parent directories do not exist [#1652](https://github.com/devfile/devworkspace-operator/pull/1652)
- Apply workspace `podSecurityContext` to PVC cleanup Job pods to match workspace deployment behavior [#1638](https://github.com/devfile/devworkspace-operator/pull/1638)
- Increase default `init-persistent-home` container memory limit from 128Mi to 256Mi and request from 64Mi to 128Mi to prevent OOM failures with large developer images [#1644](https://github.com/devfile/devworkspace-operator/pull/1644)
- Mount an `emptyDir` home volume for ephemeral workspaces when a custom `init-persistent-home` init container is configured and `persistUserHome` is enabled [#1649](https://github.com/devfile/devworkspace-operator/pull/1649)

# v0.41.0

## Features

### Mount automount resources only to specific DevWorkspaces [#1619](https://github.com/devfile/devworkspace-operator/pull/1619)

Two new annotations allow administrators to control which DevWorkspaces receive automounted ConfigMaps, Secrets, and PVCs by workspace name pattern:

- `controller.devfile.io/mount-to-devworkspace-include`: mount the resource only to workspaces whose names match the specified pattern
- `controller.devfile.io/mount-to-devworkspace-exclude`: mount the resource to all workspaces except those whose names match the specified pattern

When either annotation is set, the DevWorkspace operator also watches the annotated resource and triggers reconciliation of the targeted workspaces when the resource changes.

See [docs/additional-configuration.md](docs/additional-configuration.adoc#automatically-mounting-volumes-configmaps-and-secrets) for more details.

### Prevent workspace restart when automount resources are created or modified [#1533](https://github.com/devfile/devworkspace-operator/pull/1533)

A new `controller.devfile.io/mount-on-start` annotation can be set on automounted ConfigMaps, Secrets, and PVCs. When this annotation is set to `"true"`, adding or modifying the resource will not immediately restart running workspaces. The resource will be mounted the next time the workspace starts.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    controller.devfile.io/mount-as: env
    controller.devfile.io/mount-on-start: "true"
  labels:
    controller.devfile.io/mount-to-devworkspace: "true"
    controller.devfile.io/watch-configmap: "true"
  name: my-config
```

See [docs/additional-configuration.md](docs/additional-configuration.adoc#automatically-mounting-volumes-configmaps-and-secrets) for more details.

### Mount PVC subdirectory into workspace using subPath [#1595](https://github.com/devfile/devworkspace-operator/pull/1595)

Automounted PVCs now support mounting a subdirectory within the PVC using the `subPath` field. This is configured by providing a JSON array as the value of the `controller.devfile.io/mount-path` annotation, where each entry specifies both a `path` (mount point in the container) and a `subPath` (subdirectory within the PVC):

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    controller.devfile.io/mount-path: '[{"path":"/var/logs","subPath":"data/logs"},{"path":"/etc/config","subPath":"data/config"}]'
  labels:
    controller.devfile.io/mount-to-devworkspace: "true"
  name: my-pvc
```

## Bug Fixes & Improvements

- Conditionally copy backup registry auth secret to workspace namespace only when configured; user-provided secrets take precedence [#1618](https://github.com/devfile/devworkspace-operator/pull/1618)

- Update Go to 1.25.9 [#1617](https://github.com/devfile/devworkspace-operator/pull/1617)

# v0.40.1

## Features

### Project clone retries [#1613](https://github.com/devfile/devworkspace-operator/pull/1613)

The project clone init container now retries failed clone operations up to 3 times (4 total attempts) using exponential backoff. This improves reliability when transient network issues cause clone failures.

## Bug Fixes & Improvements

- Fix restore failing when backup auth secret name did not match the copied secret name [#1614](https://github.com/devfile/devworkspace-operator/pull/1614)

- Update Go to 1.25.8 [#1611](https://github.com/devfile/devworkspace-operator/pull/1611)

- Change project-backup image base from ubi10 to ubi9 to align with downstream image [#1610](https://github.com/devfile/devworkspace-operator/pull/1610)


# v0.40.0

## Features

### Per-workspace backup status tracking [#1549](https://github.com/devfile/devworkspace-operator/issues/1549)

The backup controller now tracks backup status individually for each DevWorkspace using annotations. When a backup job completes or fails, the following annotations are set on the DevWorkspace:

- `controller.devfile.io/last-backup-time`: timestamp of the last backup attempt
- `controller.devfile.io/last-backup-successful`: whether the last backup succeeded

This allows the controller to determine when each workspace needs a new backup based on its own backup history rather than a global timestamp.

### Restore workspace from backup [#1525](https://github.com/devfile/devworkspace-operator/issues/1525)

DevWorkspaces can now be restored from a backup by setting the `controller.devfile.io/restore-workspace: 'true'` attribute. When this attribute is set, the workspace deployment includes a restore init container that pulls the backed-up `/projects` content from an OCI registry instead of cloning from Git.

By default, the restore source is derived from the admin-configured registry at `<registry>/<namespace>/<workspace>:latest`. Users can optionally specify a custom source image using the `controller.devfile.io/restore-source-image` attribute.

```yaml
kind: DevWorkspace
spec:
  template:
    attributes:
      controller.devfile.io/restore-workspace: 'true'
      # Optional: restore from a specific image instead of the default backup registry
      controller.devfile.io/restore-source-image: 'registry.example.com/my-backup:latest'
```

### Configurable backup job retry limit [#1579](https://github.com/devfile/devworkspace-operator/issues/1579)

Administrators can now configure the maximum number of retries for backup jobs in the DevWorkspaceOperatorConfig via the `backoffLimit` field. The default value is 1.

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
config:
  workspace:
    backupCronJob:
      backoffLimit: 5
```

### Inject HOST_USERS environment variable for user namespace configuration [#1582](https://github.com/devfile/devworkspace-operator/pull/1582)

When `hostUsers` is set to `false` in the workspace configuration, the `HOST_USERS=false` environment variable is now automatically injected into workspace containers. This allows container images to detect that they are running in a dedicated user namespace and adjust their behavior accordingly.

## Bug Fixes & Improvements

- Sort automount configmaps and secrets to ensure deterministic ordering [#1578](https://github.com/devfile/devworkspace-operator/pull/1578)
- Set owner reference for image-builder rolebindings to enable cleanup on DevWorkspace deletion [#1590](https://github.com/devfile/devworkspace-operator/pull/1590)
- Fix trailing slashes in registry path causing malformed image references [#1592](https://github.com/devfile/devworkspace-operator/pull/1592)
- Update Go to 1.25.7 [#1574](https://github.com/devfile/devworkspace-operator/pull/1574)

# v0.39.0
## Features
### Implement backup feature for DevWorkspaces [#1524](https://github.com/devfile/devworkspace-operator/issues/1524)
An automated backup mechanism is now available for non-ephemeral DevWorkspaces. The backup feature can be configured through the DevWorkspaceOperatorConfig to periodically backup workspace `/project` content.

The backup process:
- Identifies stopped DevWorkspaces from a configurable time period
- Creates a Job in the user's namespace to generate a container image containing `/projects` from the DevWorkspace's PVC
- Pushes the backup image to an external image registry

See [docs/dwo-configuration.md](docs/dwo-configuration.md#configuring-backup-cronjob) for configuration details.

### Add the ability to configure custom init containers [#1559](https://github.com/devfile/devworkspace-operator/issues/1559)
Cluster administrators can now configure custom init containers for DevWorkspaces in the DevWorkspaceOperatorConfig.

Example use cases are:
- Injecting custom configuration and tools
- Overriding the built-in `init-persistent-home` logic by providing a custom container with the same name

Example configuration:
```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
config:
  workspace:
    initContainers:
      - name: install-tools
        image: custom-image:latest
        command: ["/bin/sh", "-c"]
        args:
          - |
            echo "Setting up custom tools"
            mkdir -p /home/user/custom-tools
```
See [docs/dwo-configuration.md](docs/dwo-configuration.md#configuring-custom-init-containers) for configuration details.
### Add container resource caps enforcement [#1561](https://github.com/devfile/devworkspace-operator/issues/1561)
Administrators can now set maximum resource limits and requests for workspace containers through the DevWorkspaceOperatorConfig. This prevents users from creating DevWorkspaces with excessive CPU or memory requirements.

Example configuration:
```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
config:
  workspace:
    containerResourceCaps:
      limits:
        cpu: "1"
        memory: 2Gi
      requests:
        cpu: "0.1"
        memory: 100Mi
```

When a container specifies resource requirements exceeding the caps, they will be limited to the configured maximum values. 

## Bug Fixes & Improvements
- Fix project clone image missing UID 1234 in /etc/passwd [#1560](https://github.com/devfile/devworkspace-operator/issues/1560)

# v0.38.0
## Features
### Improved debugging for failing postStart commands [#1538](https://github.com/devfile/devworkspace-operator/issues/1538)
Previously, if a postStart command failed, the container would often crash and enter CrashLoopBackOff loop, making it difficult to debug the reason for the postStart command failure.

With this release, when the `controller.devfile.io/debug-start: "true"` annotation is set on a failing DevWorkspace, any failure in a postStart command will cause the container to sleep for a configured duration (based on `config.workspace.progressTimeout` in the DevWorkspaceOperatorConfig) instead of terminating.

This gives the opportunity exec into the failing container and inspect logs in the `/tmp/poststart-stdout.txt` and `/tmp/poststart-stderr.txt` files to determine the root cause of the failure.

## Bug Fixes & Improvements
- Set readOnlyRootFilesystem for deployments to true [#1534](https://github.com/devfile/devworkspace-operator/pull/1534)
- Make container status check less restrictive [#1528](https://github.com/devfile/devworkspace-operator/pull/1528)
- Increase default per-workspace PVC size from 5Gi to 10Gi [#1514](https://github.com/devfile/devworkspace-operator/pull/1514)

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
    postStartTimeout: '30s'
```
By default, this timeout is disabled.

## Bug Fixes & Improvements
- Add E2E test to verify that DevWorkspace contents are persisted during restarts [#1467](https://github.com/devfile/devworkspace-operator/issues/1467)
- make run fails on m4 MacBook [#1494](https://github.com/devfile/devworkspace-operator/issues/1494)
- Upgrade Go toolchain to 1.24.6 [#1509](https://github.com/devfile/devworkspace-operator/issues/1509)
- Remove group writable permissions from container images [#1516](https://github.com/devfile/devworkspace-operator/issues/1516)

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
