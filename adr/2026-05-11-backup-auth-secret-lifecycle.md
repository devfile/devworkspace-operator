# Backup Auth Secret Must Not Be Owned by Any Workspace

**Status**: Accepted
**Date**: 2026-05-11
**Deciders**: DevWorkspace Operator maintainers
**Related Issue**: [CRW-10760](https://redhat.atlassian.net/browse/CRW-10760)

## Context

The backup system copies the registry authentication secret (e.g., quay.io credentials) from the operator namespace into each workspace namespace as `devworkspace-backup-registry-auth`. The original implementation set a Kubernetes controller ownerReference from this secret to the DevWorkspace that triggered the copy:

```go
controllerutil.SetControllerReference(workspace, desiredSecret, scheme)
```

This was likely intended to clean up the secret when no workspaces need it anymore — standard Kubernetes garbage collection pattern.

However, two properties of this secret make ownerReference-based lifecycle management incorrect:

1. **The secret is a namespace singleton**: All workspaces in a namespace share the same `devworkspace-backup-registry-auth` secret, but a Kubernetes object can have only one controller owner. Whichever workspace's backup job ran last "wins" ownership.

2. **The secret is needed after all workspaces are deleted**: The primary restore use case is creating a new workspace from a backup of a deleted one. If the auth secret is garbage-collected when the last workspace is deleted, the user cannot authenticate to the private registry to pull the backup image.

**Bug observed (CRW-10760)**: When using quay.io (private registry) for backups, deleting a workspace caused backup entries to disappear from the Dashboard backup list for ALL workspaces in the namespace. The auth secret was garbage-collected, and the Dashboard could no longer query the registry.

**Validated on CRC cluster** (DWO 0.40.1, quay.io/okurinny):
- The `devworkspace-backup-registry-auth` secret was confirmed to have ownerReference to a single workspace (`nodejs`)
- Deleting that workspace triggered K8s GC, removing the secret
- The secret was not re-created by subsequent backup cycles (remaining workspaces already had recent backups)
- Backup listing in the Dashboard failed for all workspaces

## Decision

**Remove the ownerReference from the backup registry auth secret.** The secret becomes a namespace-scoped resource with no ownership tie to any workspace.

### What Changes

- `pkg/secrets/backup.go`: Remove the `controllerutil.SetControllerReference()` call in `CopySecret()`
- The secret is still created via `c.Create()` with `AlreadyExists` handling, just without an ownerReference
- The restore path now resolves the operator namespace via `infrastructure.GetNamespace()` to copy the secret on demand when it is missing

### What Doesn't Change

- Per-workspace resources (job runner ServiceAccount, image-builder RoleBinding) retain their ownerReferences — their GC on workspace deletion is correct and expected
- The backup Job itself retains its ownerReference (short-lived, TTL-cleaned)
- The `CopySecret` function signature stays the same

## Considered Alternatives

### Alternative 1: Multi-owner references (non-controller)

Add each workspace as a non-controller owner of the secret. K8s GC would only delete the secret when ALL owning workspaces are gone.

**Rejected because**:
- The secret must survive deletion of ALL workspaces (for restore)
- Adds complexity to track and merge owner lists
- Non-controller ownerReferences have subtle GC semantics

### Alternative 2: Finalizer-based cleanup

Remove ownerReference but add a cleanup mechanism (e.g., a controller that deletes the secret when the namespace has zero DevWorkspaces).

**Rejected because**:
- Adds complexity for a marginal benefit (one small secret in an otherwise-empty namespace)
- Could race with restore operations (user creates a workspace from backup right after the last one is deleted)
- Namespace deletion already cleans up all resources

### Alternative 3: Per-workspace auth secrets

Create a unique auth secret per workspace (e.g., `devworkspace-backup-registry-auth-{workspace-id}`).

**Rejected because**:
- Multiplies secrets unnecessarily (all contain the same credentials)
- The restore path expects the predefined name `devworkspace-backup-registry-auth`
- Still wouldn't survive workspace deletion for restore use case

## Consequences

### Positive

1. **Backups survive workspace deletion**: Users can delete all workspaces and still restore from backups
2. **No cross-workspace interference**: Deleting one workspace no longer affects other workspaces' backup capabilities
3. **Simpler lifecycle**: No ownership tracking needed for a namespace-scoped singleton

### Negative

1. **Secret persists in empty namespaces**: If all workspaces are deleted and the user never restores, the auth secret remains until the namespace is deleted. This is a minor leak — one small secret per namespace.

### Neutral

1. **Existing secrets on upgraded clusters**: Secrets created by older DWO versions will retain their stale ownerReference until the next `CopySecret` call overwrites them (via `SyncObjectWithCluster`). In the worst case, one more GC event occurs before the fix takes effect.

## References

- `pkg/secrets/backup.go` — `CopySecret()` function
- `controllers/backupcronjob/rbac.go` — Per-workspace SA/RoleBinding (unchanged)
- `pkg/constants/metadata.go:204` — `DevWorkspaceBackupAuthSecretName`
