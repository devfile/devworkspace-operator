## DevWorkspace Operator Configuration

The DevWorkspace Operator installs the `DevWorkspaceOperatorConfig` Custom Resource Definition (short name: `dwoc`).

### Controller configuration

A `DevWorkspaceOperatorConfig` Custom Resource defines the behavior of the DevWorkspace Operator Controller.

To see documentation on configuration settings, including default values, use `kubectl explain` or `oc explain` -- e.g. 
`kubectl explain dwoc.config.routing.clusterHostSuffix`.

**The only required configuration setting is `.routing.clusterHostSuffix`, which is required when running on 
Kubernetes.**

Configuration settings in the `DevWorkspaceOperatorConfig` override default values found in [pkg/config](https://github.com/devfile/devworkspace-operator/tree/main/pkg/config). 

### Global configuration for the DevWorkspace Operator

To configure global behavior of the DevWorkspace Operator, create a `DevWorkspaceOperatorConfig` named 
`devworkspace-operator-config` in the same namespace where the operator is deployed:
```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  # Configuration fields
```

### DevWorkspace specific configuration 

To apply a configuration to a specific `DevWorkspace` instead of globally, an existing `DevWorkspaceOperatorConfig` can
be referenced in a `DevWorkspace`'s attributes:
```yaml
apiVersion: workspace.devfile.io/v1alpha2
kind: DevWorkspace
metadata:
  name: my-devworkspace
spec:
  template:
    attributes:
      controller.devfile.io/devworkspace-config:
        name: <name of DevWorkspaceOperatorConfig CR>
        namespace: <namespace of DevWorkspaceOperatorConfig CR>
```
Configuration specified as above will be merged into the default global configuration, overriding any values present.

## Configuring the Webhook deployment
The `devworkspace-webhook-server` deployment can be configured in the global `DevWorkspaceOperatorConfig`. 
The configuration options include: 
[replicas](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#replicas),
[pod tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) and
[nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).

These configuration options exist in the **global** DWOC's `config.webhook`  field:

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  webhook:
    nodeSelector:  <string, string>
    tolerations: <[]tolerations>
    replicas: <int32>
```

**Note:** In order for the `devworkspace-webhook-server` configuration options to take effect:

- You must place them in the
[Global configuration for the DevWorkspace Operator](#global-configuration-for-the-devworkspace-operator), which has the
name `devworkspace-operator-config` and exists in the namespace where the DevWorkspaceOperator is installed. If it does
not already exist on the cluster, you must create it.
- You'll need to terminate the `devworkspace-controller-manager` pod so that the replicaset can recreate it. The new pod
will update the `devworkspace-webhook-server` deployment.

## Configuring Cleanup CronJob

The DevWorkspace cleanup job helps manage resources by removing DevWorkspaces that have not been started for a configurable period of time.

**Note:** By default, the DevWorkspace cleanup job is disabled.

You can control the behaviour of the DevWorkspace cleanup job through the `config.workspace.cleanupCronJob` section of the global DWOC::

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  workspace:
    cleanupCronJob:
      enabled: true
      dryRun: false
      retainTime: 2592000
      schedule: "0 0 1 * *"
```

Cleanup CronJob configuration fields:

- **`enable`**: Set to `true` to enable the cleanup job, `false` to disable it. Default: `false`.
- **`schedule`**: A Cron expression defining how often the cleanup job runs. Default: `"0 0 1 * *"` (first day of the month at midnight).
- **`retainTime`**: The duration time in seconds since a DevWorkspace was last started before it is considered stale and eligible for cleanup. Default: 2592000 seconds (30 days).
- **`dryRun`**: Set to `true` to run the cleanup job in dry-run mode. In this mode, the job logs which DevWorkspaces would be removed but does not actually delete them. Set to `false` to perform the actual deletion. Default: `false`.

## Configuring Backup CronJob

The DevWorkspace backup job allows for periodic backups of DevWorkspace data to a specified backup location.
Once enabled and configured, the backup job will run at defined intervals to create backups of DevWorkspace data.
The backup controller depends on an OCI-compatible registry e.g., [quay.io](https://quay.io/) used as an image artifact storage for backup archives.

The backup makes a snapshot of Workspace PVCs and stores them as tar.gz archives in the specified OCI registry.
**Note:** By default, the DevWorkspace backup job is disabled.


Backup CronJob configuration fields:

- **`enable`**: Set to `true` to enable the backup job, `false` to disable it. Default: `false`.
- **`schedule`**: A Cron expression defining how often the backup job runs. Default: `"0 1 * * *"`.
- **`registry.path`**: A base registry location where the backup archives will be pushed.
The value provided for registry.path is only the first segment of the final location. The full registry path is assembled dynamically, incorporating the name of the workspace and the :latest tag, following this pattern:
`<registry.path>/<devworkspace-name>:latest`

- **`registry.authSecret`**: (Optional) The name of the Kubernetes Secret containing credentials to access the OCI registry. If not provided, it is assumed that the registry is public or uses integrated OpenShift registry.
- **`oras.extraArgs`**: (Optional) Additional arguments to pass to the `oras` CLI tool during push and pull operations.


There are several configuration options to customize the logic:

### Integrated OpenShift container registry
This option is available only on OpenShift clusters with integrated container registry enabled and requires no additional configuration.

To enable the backup use following configuration in the global DWOC:

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
config:
  routing:
    defaultRoutingClass: basic
  workspace:
    backupCronJob:
      enable: true
      registry:
        path: default-route-openshift-image-registry.apps.{cluster ID}.openshiftapps.com
      schedule: '0 */4 * * *' # cron expression with backup frequency
    imagePullPolicy: Always
```

**Note:** The `path` field must contain the URL to your OpenShift integrated registry given by the cluster.

Once the backup job is finished the backup archives will be available in the DevWorkspace namespace under a repository
with matching Devworkspace name.

### Regular OCI compatible registry
To use a regular OCI compatible registry for backups, you need to provide registry credentials. Depending on your
RBAC policy the token can be provided via a secret in the operator namespace or in each DevWorkspace namespace.
Having the secret in the DevWorkspace namespace allows for using different registry accounts per namespace with more
granular access control.


```yaml
kind: DevWorkspaceOperatorConfig
apiVersion: controller.devfile.io/v1alpha1
config:
  routing:
    defaultRoutingClass: basic
  workspace:
    backupCronJob:
      enable: true
      registry:
        authSecret: my-secret
        path: quay.io/my-company-org
      schedule: '0 */4 * * *'
    imagePullPolicy: Always
```
The `authSecret` must point to real Kubernetes Secret of type `kubernetes.io/dockerconfigjson` containing credentials to access the registry.

To create one you can use following command:

```bash
kubectl create secret docker-registry my-secret --from-file=config.json -n devworkspace-controller
```
The secret must contain a label `controller.devfile.io/watch-secret=true` to be recognized by the DevWorkspace Operator.
```bash
kubectl label secret my-secret controller.devfile.io/watch-secret=true -n devworkspace-controller
```

### Restore from backup
We are aiming to provide automated restore functionality in future releases. But for now you can still
manually restore the data from the backup archives created by the backup job.

Since the backup archive is available in OCI registry you can use any OCI compatible tool to pull
the archive locally. For example using [oras](https://github.com/oras-project/oras) cli tool:

```bash
oras pull <registry-path>/<devworkspace-name>:latest
```
The archive will be downloaded as a `devworkspace-backup.tar.gz` file which you can extract and restore the data.

## Configuring PVC storage access mode

By default, PVCs managed by the DevWorkspace Operator are created with the `ReadWriteOnce` access mode.
The access mode can be configured with the `config.workspace.storageAccessMode` section of the global DWOC:

```yaml
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: $OPERATOR_INSTALL_NAMESPACE
config:
  workspace:
    storageAccessMode:
    - ReadWriteMany
```

The config above will have newly created PVCs to have its access mode set to `ReadWriteMany`.
