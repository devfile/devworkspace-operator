# Sanitize automount volume names to be DNS-1123 compliant

**Status**: Proposed
**Date**: 2026-08-26
**Deciders**: DevWorkspace Operator maintainers
**Related Issue**: CRW-9800

## Context

When a Secret, ConfigMap, or PVC is auto-mounted into a workspace (via the
`controller.devfile.io/mount-to-devworkspace` label), DWO derives a pod volume
name from the object's name. Previously the object name was used verbatim
(`AutoMountSecretVolumeName`, `AutoMountConfigMapVolumeName`,
`AutoMountPVCVolumeName` all returned their input unchanged).

Kubernetes object names and volume names have *different* validation rules. A
Secret named `test.pullsecret` is perfectly legal, but a pod volume name must be
a DNS-1123 label (lowercase alphanumeric plus `-`, must start/end alphanumeric,
≤63 chars). A dot is invalid in a volume name. As a result, auto-mounting a
secret whose name contained a dot (or other invalid character) produced an
invalid Deployment, and the workspace failed to start.

The object name itself is valid and must be preserved — the volume's
`secretName`/`configMap.name`/`claimName` still has to reference the real
object. Only DWO's *derivation* of the volume name was wrong.

## Decision

Sanitize the derived volume name to a DNS-1123 label via a shared
`sanitizeVolumeName` helper in `pkg/common/naming.go`, used by all three
`AutoMount*VolumeName` functions. Sanitization lowercases the name, replaces
runs of invalid characters with `-`, trims leading/trailing `-`, and truncates
to 63 characters (trimming any trailing `-` left by truncation).

The volume's reference to the underlying object (`secretName`, `configMap.name`,
`claimName`) continues to use the original, unmodified object name.

## Considered Alternatives

### Alternative 1: Reject invalid object names at admission (webhook validation)

Add a validating webhook that denies a workspace (or the labeled object) when an
auto-mount source has a name that cannot form a valid volume name.

**Rejected because**:
- The object name is legal Kubernetes; rejecting it pushes a DWO-internal
  limitation onto the user, who did nothing wrong.
- Auto-mounted objects are matched by label and can be created independently of
  (and after) the workspace, so there is no single admission point that cleanly
  owns this validation.
- It is a worse user experience: the workspace fails instead of just working.

### Alternative 2: Keep names verbatim, only truncate for length

The pre-existing behavior already tolerated long names implicitly; only add
length handling.

**Rejected because**:
- It does not fix the reported bug — invalid *characters* (dots, underscores,
  etc.), not just length, are the failure in CRW-9800.

## Consequences

### Positive

1. Auto-mounting objects with names that are legal in Kubernetes but invalid as
   volume names now works transparently.
2. Length handling (≤63 chars) is now correct as a side effect, replacing the
   previous reliance on never adding characters to the name.

### Negative

1. Sanitization is not injective: two distinct object names can map to the same
   volume name (e.g. `test.pullsecret` and `test-pullsecret`). This is an
   accepted trade-off. Because `checkAutomountVolumesForCollision` previously only
   detected DevWorkspace-vs-automount name collisions and mount-path collisions —
   not two *automounted* objects resolving to the same name — this change also
   extends that check to catch the new case, so it surfaces a clear error rather
   than producing an invalid pod spec that the API server rejects. Previously
   these names were distinct; the collision case is new but rare and fails loudly.

### Neutral

1. The old comment on `AutoMount*VolumeName` explaining why prefixes were not
   added (to avoid exceeding 63 chars) was removed, as length is now handled
   explicitly by `sanitizeVolumeName`.

## References

- `pkg/common/naming.go` — `sanitizeVolumeName` and the `AutoMount*VolumeName` functions
- `pkg/common/naming_test.go` — unit tests for sanitization
- `pkg/provision/automount/testdata/testSanitizesInvalidVolumeNames.yaml` — fixture-based integration test
- `pkg/provision/automount/testdata/errorDuplicateVolumeNameAfterSanitization.yaml` — fixture for the collision case
- `test/e2e/pkg/tests/automount_volume_sanitization_tests.go` — end-to-end test
- `pkg/provision/automount/common.go` — `checkAutomountVolumesForCollision` (extended to detect automount-vs-automount name collisions)
