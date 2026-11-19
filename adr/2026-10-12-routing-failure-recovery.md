# Recover failed routing after spec changes

**Status**: Accepted
**Date**: 2026-10-12
**Deciders**: DevWorkspace Operator maintainers

## Context

A DevWorkspaceRouting can fail because an endpoint conflicts with a Service owned by someone else or its configuration is invalid. The controller previously treated Failed as terminal even after the user corrected the routing spec. Recovery required recreating the routing object.

## Decision

Record the spec generation represented by Preparing, Ready, Failed, and Stopped in the optional `status.observedGeneration` field. A failure remains terminal for that generation. When a failed routing has a different generation, clear published Pod additions and endpoints, persist Preparing, and reconcile the new spec through the existing Service ownership and certificate validation checks. If the Preparing write fails, return before provisioning resources.

Legacy Failed objects without this field have observed generation zero. Kubernetes assigns a positive generation, so the controller reconciles them once and records the current generation. If validation fails again, the failure becomes terminal for that generation.

The workspace consumer retries a Failed status only when its nonzero observed generation differs from the routing object's current generation. Current failures and legacy failures retain their existing failure handling. This does not change workspace restart behavior.

## Considered Alternatives

### Alternative 1: Retry unchanged failures on resource events

Reconcile failed routing whenever a Service or another watched resource changes.

**Rejected because**:
- Failures were selected to remain terminal until the routing spec changes.
- Unrelated events would repeatedly run validation and could turn a terminal failure into automatic recovery without a spec correction.

### Alternative 2: Require routing recreation

Keep Failed terminal across all generations.

**Rejected because**:
- Correcting the spec should allow the same routing object to recover.
- Recreation discards routing identity and adds an unnecessary resource lifecycle step.

## Consequences

### Positive

1. A corrected spec can recover without recreating routing or modifying foreign Services.
2. Status identifies the spec generation it represents, and unchanged failures avoid repeated failure writes.

### Negative

1. Removing an external conflict alone does not recover a failed routing; its spec must change.
2. Updated CRDs must be installed before the controller so Kubernetes retains `observedGeneration`.

### Neutral

1. Legacy failures receive one reconciliation when the updated controller first observes them.
2. Existing Service ownership checks, certificate validation, and workspace restart behavior continue to apply.

## References

- [Routing status API](../apis/controller/v1alpha1/devworkspacerouting_types.go)
- [Routing controller](../controllers/controller/devworkspacerouting/devworkspacerouting_controller.go)
- [Workspace routing consumer](../pkg/provision/workspace/routing.go)
