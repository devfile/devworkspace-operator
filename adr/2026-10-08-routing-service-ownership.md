# Guard routing Service ownership during synchronization

**Status**: Accepted
**Date**: 2026-10-08
**Deciders**: Not recorded

## Context

DevWorkspaceRouting Services share namespace names, including discoverable endpoints. Admission reads cached workspace state, so concurrent requests can both pass; the cache can also hide a Service missing its workspace ID label. Treating that miss as proof a name is free risks overwriting another routing's Service.

## Decision

Call `SyncRoutingServiceWithCluster` explicitly for every routing-controller Service, including aggregate Services. Kubernetes/OpenShift component handling dispatches inlined Services with the discoverable annotation `"true"` or a DevWorkspaceRouting controller owner to the same guarded entry point. The routing ownership checks and Service creation-race behavior remain unchanged.

Ordinary inlined resources and async-storage Services use `SyncObjectWithCluster`. Its normal reads use the cached client. Only when that read returns NotFound and creation returns AlreadyExists does it make one lookup through the non-caching client to recover a possible create race. If no non-caching client is configured, the fresh lookup is still NotFound, or the object disappears before it can be read, return a retry error. A fresh read error also returns without writing. This collision-only path requires one or more desired owner references, each with a nonempty UID and a matching existing reference according to `containsOwnerRef`. This supports both non-controller DevWorkspace ownership and asynchronous Deployment ownership. An unexpected controller owner makes the Service foreign even when its workspace ID label matches. A foreign Service is a permanent Service-name conflict and is left unchanged. An owned Service is synchronized using the fresh object's UID, resourceVersion, and ClusterIP, with the normal cached writer.

The routing entry point reads through the non-caching client when configured (controller setup supplies it), falling back to the cached client otherwise, and uses that single result for ownership. Both entry points share post-read synchronization with no additional read. If the desired Service has a controller owner, its UID must match the existing controller owner's UID; that UID remains authoritative when the workspace ID label is stale or missing, allowing synchronization to repair the label. Without a desired controller owner, require a nonempty workspace ID label matching the existing Service. Reject a foreign same-name Service as a permanent conflict.

If Service creation returns AlreadyExists, retry instead of using the generic update fallback. Updates carry the existing UID, resourceVersion, and ClusterIP; cleanup deletes only a routing-controlled Service with UID and resourceVersion preconditions. These guards reject writes against a changed or replaced Service; update conflicts and missing objects trigger another reconcile. The read and ownership decision are not atomic, and this does not reserve names across reconcilers.

## Considered Alternatives

### Alternative 1: Trust admission conflict checks

Use cached admission as the sole protection for discoverable endpoint names.

**Rejected because**: Stale cache state lets concurrent requests both pass before a Service exists.

### Alternative 2: Treat workspace ID labels as ownership proof

Require the existing Service label to match the desired workspace ID.

**Rejected because**: A label match does not prove matching controller ownership, and labels can drift. A valid routing owner can repair a stale label without losing its Service.

### Alternative 3: Keep the generic AlreadyExists update fallback

Use generic update after Service creation reports AlreadyExists.

**Rejected because**: The fallback lacks a fresh object for ownership validation and identity preservation.

## Consequences

### Positive

- The fresh check protects foreign Services hidden by cache filtering.
- Owned Services with stale labels can be repaired while preserving identity and ClusterIP.

### Negative

- The routing path's fresh reads add API calls and latency; the generic collision recovery adds a fresh read only after a create collision, instead of uncached reads during routine synchronization. Changed Services can make guarded writes conflict and require another reconcile.

### Neutral

- Admission remains advisory; the controller check is authoritative. Concurrent creates rely on Kubernetes name uniqueness and retry.

## References

- [Generic synchronization](../pkg/provision/sync/sync.go)
- [Explicit routing Service synchronization](../pkg/provision/sync/service.go)
- [Inlined component dispatch](../pkg/library/kubernetes/provision.go)
- [Service update fields](../pkg/provision/sync/update.go)
- [Service conflict error](../pkg/provision/sync/service.go)
- [Routing Service cleanup](../controllers/controller/devworkspacerouting/sync_services.go)
- [Routing controller client setup](../controllers/controller/devworkspacerouting/devworkspacerouting_controller.go)
- [Ownership and cache-miss tests](../pkg/provision/sync/service_test.go)
- [Routing cleanup and conflict tests](../controllers/controller/devworkspacerouting/sync_services_test.go)
- [Label repair tests](../controllers/controller/devworkspacerouting/workspace_name_test.go)
- [Admission index decision](2026-10-07-discoverable-endpoint-index.md)
