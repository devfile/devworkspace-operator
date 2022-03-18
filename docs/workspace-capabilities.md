# Default DevWorkspace cluster permissions

This document outlines the permissions that are provisioned for the DevWorkspace service account by default. The role attached to the workspace ServiceAccount is defined in [rbac.go](../pkg/provision/workspace/rbac.go). As a regular Kubernetes role definition, this is
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: workspace
rules:
- apiGroups:
  - ""
  resources:
  - pods/exec
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - apps
  - extensions
  resources:
  - deployments
  - replicasets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resourceNames:
  - workspace-credentials-secret
  resources:
  - secrets
  verbs:
  - get
  - create
  - patch
  - delete
- apiGroups:
  - ""
  resourceNames:
  - workspace-preferences-configmap
  resources:
  - configmaps
  verbs:
  - get
  - create
  - patch
  - delete
- apiGroups:
  - workspace.devfile.io
  resources:
  - devworkspaces
  verbs:
  - get
  - watch
  - list
  - patch
  - update
- apiGroups:
  - controller.devfile.io
  resources:
  - devworkspaceroutings
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - workspace.devfile.io
  resources:
  - devworkspacetemplates
  verbs:
  - get
  - create
  - patch
  - update
  - delete
  - list
  - watch
```

Additional permissions can be bound to the DevWorkspace ServiceAccount as follows:

1. Find the *DevWorkspace ID* for the DevWorkspace in question. This is available on the `.status.devworkspaceId` field in the object, which can be obtained using `jq`:
    ```bash
    DEVWORKSPACE_ID=$(kubectl get devworkspaces <workspace-name> -o json | jq -r '.status.devworkspaceId')
    ```
    The service account created for the DevWorkspace will be named `${DEVWORKSPACE_ID}-sa`
2. Create a rolebinding to attach a custom role to the workspace service account:
    ```yaml
    NAMESPACE=<workspace namespace>
    cat << EOF | kubectl apply -f -
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: workspace-${DEVWORKSPACE_ID}-custom-binding
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: Role
      name: <custom role name>
    subjects:
    - kind: ServiceAccount
      name: ${DEVWORKSPACE_ID}-sa
      namespace: $NAMESPACE
    EOF
    ```
    where `<current namespace>` is the namespace of the workspace, and `<custom role name>` is the name of role to be bound to the DevWorkspace.

In order to grant permissions to all workspaces in a given namespace, the RoleBinding can instead bind to all ServiceAccounts in the namespace:
```yaml
NAMESPACE=<workspace namespace>
cat << EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: devworkspace-custom-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: <custom role name>
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts:$NAMESPACE
EOF
```
Note however that this will bind the role to _all_ service accounts in that namespace, including the default serviceaccount used for pods.

For more information on creating rolebindings, see [rolebinding and clusterrolebinding](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#rolebinding-and-clusterrolebinding) in the Kubernetes documentation.
