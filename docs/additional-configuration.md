# Additional configuration for DevWorkspaces

DevWorkspaces allow for additional configuration through the use Devfile attributes and Kubernetes labels and annotations. There are three fields in the DevWorkspace that are used to configure functionality:
* Devfile attributes defined within the DevWorkspace spec, either at the *top-level* (`.spec.template.attributes`) or on *individual components*. For example:
    ```yaml
    kind: DevWorkspace
    apiVersion: workspace.devfile.io/v1alpha2
    metadata:
      name: my-workspace
    spec:
      template:
        attributes:
          my-attribute: my-attribute-value
    ```
* Kubernetes metadata labels (`.metadata.labels`):
    ```yaml
        kind: DevWorkspace
        apiVersion: workspace.devfile.io/v1alpha2
        metadata:
          name: my-workspace
          labels:
            my-label: my-label-value
    ```
* Kubernetes metadata annotations (`.metadata.annotations`):
    ```yaml
        kind: DevWorkspace
        apiVersion: workspace.devfile.io/v1alpha2
        metadata:
          name: my-workspace
          labels:
            my-annotation: my-annotation-value
    ```

## Restricting access to DevWorkspaces
Applying the
```yaml
controller.devfile.io/restricted-access: "true"
```
annotation to a DevWorkspace enables additional access control for the workspace. When this annotation is applied to a DevWorkspace:
* Only the user that created the DevWorkspace can access a terminal in the workspace via `pods/exec`
* Only the DevWorkspace Operator serviceaccount or the user that created the DevWorkspace can modify fields in the DevWorkspace custom resource.

This is useful in case a DevWorkspace is expected to contain sensitive information.


## Configuring persistent storage used for a DevWorkspace
The top-level Devfile attribute `controller.devfile.io/storage-type` can be used to configure persistent storage for DevWorkspaces:
```yaml
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: my-workspace
spec:
  template:
    attributes:
      controller.devfile.io/storage-type: <storage-type>
```
where `<storage-type>` is one of
- `common`: Use one PVC for all workspace volumes, mounting Devfile volumes in subpaths within the common PVC
- `ephemeral`: Replace all volumes with `emptyDir` volumes. This storage type is non-persistent; any local changes will be lost when the workspace is stopped. This is the equivalent of marking all volumes in the Devfile as `ephemeral: true`
- `async`: Use `emptyDir` volumes for workspace volumes, but include a sidecar that synchronises local changes to a persistent volume as in the `common` strategy. This can potentially avoid issues where mounting volumes to a workspace on startup takes a long time.

## Configuring project cloning
The top-level Devfile attribute `controller.devfile.io/project-clone` can be used to configure how storage is mounted to workspaces. By default, the DevWorkspace Operator will add an init container to the workspace deployment that will clone any projects to the workspace before start. This can be disabled by setting `controller.devfile.io/project-clone: disable` in the attributes field:
```yaml
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: my-workspace
spec:
  template:
    attributes:
      controller.devfile.io/project-clone: disable
```

## Automatically mounting volumes, configmaps, and secrets
Existing configmaps, secrets, and persistent volume claims on the cluster can be configured by applying the appropriate labels. To mark a resource for mounting to workspaces, apply the **label**
```yaml
metadata:
  labels:
    controller.devfile.io/mount-to-devworkspace: "true"
```
to the resource. For secrets and configmaps, it's also necessary to apply an additional **label**:
* `controller.devfile.io/watch-configmap` must be applied to configmaps to enable the DevWorkspace Operator to find them on the cluster
* `controller.devfile.io/watch-secret` must be applied to secrets to enable the DevWorksapce Operator to find them on the cluster.

By default, resources will be mounted based on the resource name:
* Secrets will be mounted to `/etc/secret/<secret-name>`
* Configmaps will be mounted to `/etc/config/<configmap-name>`
* Persistent volume claims will be mounted to `/tmp/<pvc-name>`

Mounting resources can be additionally configured via **annotations**:
* `controller.devfile.io/mount-path`: configure where the resource should be mounted
* `controller.devfile.io/mount-as`: for secrets and configmaps only, configure how the resource should be mounted to the workspace
    * If `controller.devfile.io/mount-as: file`, the configmap/secret will be mounted as files within the mount path. This is the default behavior.
    * If `controller.devfile.io/mount-as: subpath`, the keys and values in the configmap/secret will be mounted as files within the mount path using subpath volume mounts.
    * If `controller.devfile.io/mount-as: env`, the keys and values in the configmap/secret will be mounted as environment variables in all containers in the DevWorkspace.

  When "file" is used, the configmap is mounted as a directory within the workspace, erasing any files/directories already present. When "subpath" is used, each key in the configmap/secret is mounted as a subpath volume mount in the mount path, leaving existing files intact but preventing changes to the secret/configmap from propagating into the workspace without a restart.
* `controller.devfile.io/read-only`: for persistent volume claims, mount the resource as read-only

## Adding image pull secrets to workspaces
Labelling secrets with `controller.devfile.io/devworkspace_pullsecret: true` marks a secret as the Docker pull secret for the workspace deployment. This should be applied to secrets with docker config types (`kubernetes.io/dockercfg` and `kubernetes.io/dockerconfigjson`)

Note: As for automatically mounting secrets, it is necessary to apply the `controller.devfile.io/watch-secret` label to image pull secrets

## Adding git credentials to a workspace
Labelling secrets with `controller.devfile.io/git-credential` marks the secret as containing git credentials. All git credential secrets will be merged into a single secret (leaving the original resources in-tact). See [git documentation](https://git-scm.com/docs/git-credential-store#_storage_format) for details on the file format for this configuration. For example
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: git-credentials-secret
  annotations:
    controller.devfile.io/mount-path: /home/theia/.git-credentials/
  labels:
    controller.devfile.io/git-credential: 'true'
type: Opaque
data:
  credentials: https://{USERNAME}:{PERSONAL_ACCESS_TOKEN}@{GIT_WEBSITE}
```

Note: As for automatically mounting secrets, it is necessary to apply the `controller.devfile.io/watch-secret` label to git credentials secrets

## Debugging a failing workspace
Normally, when a workspace fails to start, the deployment will be scaled down and the workspace will be stopped in a `Failed` state. This can make it difficult to debug misconfiguration errors, so the annotation `controller.devfile.io/debug-start: "true"` can be applied to DevWorkspaces to leave resources for failed workspaces on the cluster. This allows viewing logs from workspace containers.
