# Validate serving certificates before publishing routing mounts

**Status**: Accepted
**Date**: 2026-10-12
**Deciders**: DevWorkspace Operator maintainers

## Context

Cluster TLS routing requests one serving-certificate Secret per Service, including discoverable endpoint Services. An existing Secret with the requested name can belong to another Service or contain unrelated data. Publishing its mount before verifying its identity can expose that data to workspace containers. The [certificate naming decision](2026-10-08-discoverable-service-certificates.md) remains applicable.

## Decision

After Service synchronization, built-in `cluster-tls` and `web-terminal` routing verifies every requested certificate Secret through the non-caching client before becoming ready. Require a nonempty synchronized Service UID, matching originating Service name/UID annotations (beta or legacy alpha, without conflicting values), a matching core Service owner reference, TLS Secret type, and nonempty `tls.crt` and `tls.key`. The owner reference need not be a controller reference, matching service-ca's generated ownership.

Missing Secrets or TLS data keep routing Preparing and requeue after one second. Continue checking other Services so a foreign Secret collision still fails routing. Identity or type mismatches fail routing; lookup errors remain retryable. Pending or failed routing clears published Pod additions and endpoints. The operator never changes, adopts, or deletes these Secrets.

Keep Secret names, volume names, permissions, and mount paths. Project only `tls.crt` and `tls.key` into certificate volumes. Keep the label-filtered Secret cache, public APIs, plaintext routing, and custom solvers unchanged.

## Considered Alternatives

### Alternative 1: Change certificate Secret names

Use workspace-specific names to reduce collisions.

**Rejected because**: Naming is an existing contract, and changing it would require migration while still leaving the need to validate any preexisting Secret at the new name.

### Alternative 2: Publish mounts immediately after Service creation

Let service-ca and pod volume mounting determine readiness.

**Rejected because**: A preexisting foreign Secret can satisfy volume mounting without being a certificate for the synchronized Service.

## Consequences

### Positive

1. Foreign Secrets cannot be published as certificate mounts merely because their names match.
2. Extra Secret keys are excluded from certificate volumes.

### Negative

1. Each TLS routing reconciliation adds an authoritative Secret read per requested certificate.
2. Workspace startup waits for service-ca to issue certificates, with up to one second of polling delay after issuance.
3. A privileged Secret writer can replace or change a Secret after validation; this readiness gate does not prevent that race.

### Neutral

1. Local envtest and live DWO verification completed on 2026-10-13. Checks covered a foreign Secret remaining unchanged and unpublished as a mount, certificate identity tied to the synchronized Service UID, service-ca pause and recovery in 2.754 seconds (within the 60-second bound), and 24/24 live e2e checks with the documented configuration workaround. The matching upstream DWO CI result remains pending until PR review; Che verification is deferred. These live measurements used source `860011619f3815b2fbbc4d000f72e32a347f3da9`.
2. No Secret watches or changes to resource ownership are introduced.

## References

- [Routing controller](../controllers/controller/devworkspacerouting/devworkspacerouting_controller.go)
- [Cluster solver](../controllers/controller/devworkspacerouting/solvers/cluster_solver.go)
- [Service-ca collision handling and Secret identity](https://github.com/openshift/service-ca-operator/blob/main/pkg/controller/servingcert/controller/secret_creating_controller.go)
- [Service-ca Secret generation](https://github.com/openshift/service-ca-operator/blob/main/pkg/controller/servingcert/controller/serving_cert_issuer.go)
