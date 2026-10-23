# Mount per-service certificates for discoverable endpoints

**Status**: Accepted
**Date**: 2026-10-08
**Deciders**: Not recorded

## Context

With TLS enabled, cluster routing creates an aggregate workspace Service and one Service per discoverable endpoint. OpenShift issues a serving certificate for each annotated Service. Mounting all certificates at `/var/serving-cert/` overlaps mounts; clients need a distinct certificate location for each discoverable Service.

## Decision

Keep the aggregate Service certificate mounted at `/var/serving-cert/`. Mount each discoverable Service's certificate at `/var/serving-cert/<service-name>/`. The solver annotates each Service with its own name, and the Secret reference continues to use that actual Service name.

Keep `devworkspace-serving-cert-<service-name>` when the full generated volume name fits within 63 characters; otherwise use `devworkspace-cert-` plus the hex encoding of the first 10 SHA-256 bytes of the Service name. Distinct prefixes prevent a digest name from deterministically colliding with one generated for a short Service name. Hashing avoids truncation collisions; the 80-bit digest can still collide and is not guaranteed unique. Service and Secret names remain unchanged.

## Considered Alternatives

### Alternative 1: Share the aggregate certificate mount

Expose the aggregate certificate to workloads and use it for discoverable Service connections.

**Rejected because**: OpenShift binds each serving certificate to its Service's internal DNS name. Reusing the aggregate certificate would leave the discoverable Service DNS names uncovered. [OpenShift certificate documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/security_and_compliance/configuring-certificates)

### Alternative 2: Truncate long volume names or reject long Service names

Shorten generated names to fit Kubernetes' 63-character volume-name limit, or disallow endpoint names that produce longer values.

**Rejected because**: Truncation can merge names from distinct Services, while rejecting them would disallow otherwise valid Service names.

### Alternative 3: Hash every volume name

Use a fixed-length digest-based name for all Services.

**Rejected because**: It removes readable names for ordinary cases without avoiding any additional limit.

## Consequences

### Positive

- Each discoverable Service has a distinct certificate mount path alongside the unchanged aggregate mount.
- Long valid Service names can be represented by Kubernetes-compatible volume names without truncation.

### Negative

- Each discoverable Service adds a pod volume and mount.
- The 80-bit digest has a theoretical collision risk for distinct long Service names.

### Neutral

- Workloads use the actual Service name to identify the corresponding Secret; only the generated volume name may differ for long names.
- The reserved-name guard prevents a discoverable Service from taking the aggregate Service name, and duplicate normalized Service names are rejected.

## References

- [Cluster solver](../controllers/controller/devworkspacerouting/solvers/cluster_solver.go)
- [Cluster solver certificate mount tests](../controllers/controller/devworkspacerouting/solvers/cluster_solver_test.go)
- [Serving certificate volume naming](../pkg/common/naming.go)
- [Discoverable Service name guard](../controllers/controller/devworkspacerouting/solvers/common.go)
