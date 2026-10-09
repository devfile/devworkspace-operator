# Index discoverable endpoints for admission conflict checks

**Status**: Accepted
**Date**: 2026-10-07
**Deciders**: Not recorded

## Context

Admission checks for a workspace with discoverable endpoints currently list every DevWorkspace in its namespace, then scan the returned specs for colliding Service names. This copies and examines unrelated workspaces. The existing `discoverableEndpointNames` helper already extracts normalized, unique endpoint names when the discoverable attribute is boolean or string `true` and exposure is not `none`.

## Decision

Register a controller-runtime field index on DevWorkspace objects under `controller.devfile.io/discoverable-endpoint`. Its extractor reuses `discoverableEndpointNames`. Register the index before webhook handlers are registered and before the manager starts.

For each incoming normalized discoverable endpoint name, admission performs one namespace-scoped List using that exact indexed name. Keep the existing update gating, cached-client copy behavior, self-UID exclusion, error handling, and defensive endpoint scan of returned candidates. The scan remains the final admission check for a matching candidate.

The index covers endpoints visible in `.spec.template`; contributed endpoints remain protected by the controller's Service synchronization ownership check. Admission remains eventually consistent, so concurrent requests can both pass; the controller's fresh Service ownership check remains authoritative. No API, RBAC, dependency, or code-generation changes are required.

## Considered Alternatives

### Alternative 1: Keep listing and scanning the whole namespace

This is simpler, but continues copying every workspace and scanning unrelated specs for each admission check.

**Rejected because**: The field index targets the matching workspaces through the existing cache.

### Alternative 2: Disable cache object copying

This would avoid copies for all listed workspaces but exposes shared cached objects to mutation and races.

**Rejected because**: Limiting query results with the index avoids copying unrelated workspaces while retaining the client's normal safety behavior.

### Alternative 3: Add a custom cache or external endpoint index owner

This would add another mechanism and lifecycle to maintain.

**Rejected because**: A native controller-runtime field index provides the needed lookup within the existing manager and cache.

## Consequences

### Positive

Only workspaces matching each incoming discoverable endpoint name are copied and scanned.

### Negative

The cache uses memory for the index and CPU to extract and update indexed names. Requests with multiple incoming names issue one List per name.

### Neutral

The lookup remains a best-effort admission check over cached state. Existing update gating, conflict responses, and the controller's Service conflict guard retain their roles.

## References

- [Endpoint validation](../webhook/workspace/handler/validate.go)
- [Webhook configuration](../webhook/workspace/config.go)
