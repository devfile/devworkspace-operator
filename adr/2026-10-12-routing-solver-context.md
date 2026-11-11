# Pass reconciliation context through routing solvers

**Status**: Accepted
**Date**: 2026-10-12
**Deciders**: Repository owner

## Context

Discoverable endpoint lookup performs Service reads while constructing routing objects. The reconciler receives a context, but the solver interface and lookup helper do not accept it. Using `context.TODO()` for these reads prevents caller cancellation and deadlines from reaching the Kubernetes client.

## Decision

Add `ctx context.Context` as the first parameter of `RoutingSolver.GetSpecObjects` and `GetDiscoverableServicesForEndpoints`. Both built-in solvers pass the caller context to the helper, which uses it for Service GETs. The reconciler passes its context to the solver. Keep client selection, ownership validation, and error handling unchanged; cancellation returns through the generic reconciliation-error path.

External callers must pass a context, and external `RoutingSolver` implementations must update their `GetSpecObjects` signature. Implementations performing I/O should forward that context to their clients. Standalone callers without a request lifecycle can explicitly use `context.Background()`.

## Considered Alternatives

### Alternative 1: Retain legacy entry points alongside context-aware methods

Add new context-aware methods and retain the old methods for source compatibility.

**Rejected because**: This adds dispatch and compatibility code while leaving legacy callers unable to propagate cancellation. The repository owner chose the direct signature change.

### Alternative 2: Add a fixed timeout to Service lookup

Bound each lookup without accepting the reconciliation context.

**Rejected because**: A timeout does not propagate caller cancellation or its existing deadline and introduces an unrelated timeout policy.

## Consequences

### Positive

- Discoverable Service reads can observe reconciliation cancellation, deadlines, and context values.

### Negative

- External Go callers and solver implementations must update before compiling against the new signatures.

### Neutral

- No Kubernetes API or CRD changes are required.
- Service synchronization passes the caller context to Service LIST, GET, CREATE, UPDATE, and DELETE operations. Cancellation or deadline errors stop synchronization and propagate from reconciliation without an explicit requeue or status write.
- Finalization retains its existing context handling; deletion of obsolete Services during synchronization uses the caller context.

## References

- [Solver interface](../controllers/controller/devworkspacerouting/solvers/solver.go)
- [Discoverable Service lookup](../controllers/controller/devworkspacerouting/solvers/common.go)
- [Reconciliation entry point](../controllers/controller/devworkspacerouting/devworkspacerouting_controller.go)
- [Solver cancellation tests](../controllers/controller/devworkspacerouting/solvers/context_test.go)
- [Service synchronization context tests](../controllers/controller/devworkspacerouting/service_sync_context_test.go)
- [Reconciliation Service synchronization context tests](../controllers/controller/devworkspacerouting/service_sync_reconcile_context_test.go)
