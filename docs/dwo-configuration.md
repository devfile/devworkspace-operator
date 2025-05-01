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
