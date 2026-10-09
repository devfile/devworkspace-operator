# Guard routing Service ownership during synchronization

**Status**: Accepted
**Date**: 2026-10-08
**Deciders**: Not recorded

## Context

DevWorkspaceRouting Services share namespace names, including discoverable endpoints. Admission reads cached workspace state, so concurrent requests can both pass; the cache can also hide a Service missing its workspace ID label. Treating that miss as proof a name is free risks overwriting another routing's Service.

## Decision

Identify routing Services by the discoverable annotation or DevWorkspaceRouting controller owner. When configured, read them through the non-caching client (controller setup supplies it) and use this fresh result for ownership. If the desired Service has a controller owner, its UID must match the existing controller owner's UID; that UID remains authoritative when the workspace ID label is stale or missing, allowing synchronization to repair the label. Without a desired controller owner, require a nonempty workspace ID label matching the existing Service. Reject a foreign same-name Service as a permanent conflict.

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

- Fresh reads add API calls and latency; changed Services can make guarded writes conflict and require another reconcile.

### Neutral

- Admission remains advisory; the controller check is authoritative. Concurrent creates rely on Kubernetes name uniqueness and retry.

## References

- [Service synchronization](../pkg/provision/sync/sync.go)
- [Service update fields](../pkg/provision/sync/update.go)
- [Service conflict error](../pkg/provision/sync/service.go)
- [Routing Service cleanup](../controllers/controller/devworkspacerouting/sync_services.go)
- [Routing controller client setup](../controllers/controller/devworkspacerouting/devworkspacerouting_controller.go)
- [Ownership and cache-miss tests](../pkg/provision/sync/service_test.go)
- [Routing cleanup and conflict tests](../controllers/controller/devworkspacerouting/sync_services_test.go)
- [Label repair tests](../controllers/controller/devworkspacerouting/workspace_name_test.go)
- [Admission index decision](2026-10-07-discoverable-endpoint-index.md)
