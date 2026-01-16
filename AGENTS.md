# DevWorkspace Operator - AI Agent Instructions

**Purpose**: This file provides agent-specific guidance for working on the DevWorkspace Operator. For general project information, see [README.md](README.md). For development workflow, see [CONTRIBUTING.md](CONTRIBUTING.md).

**Project**: Kubernetes Operator for Cloud Development Environments (Go, controller-runtime, Kubebuilder)

**AI Agent Note**: Always check `go.mod` for current dependency versions before suggesting code. When modifying APIs or kubebuilder markers, suggest running `make generate_all`.

## APIs Provided by DevWorkspace Operator

The DevWorkspace Operator provides four Kubernetes APIs:

1. **DevWorkspace** - Represents a cloud development workspace, closely aligned with its source Devfile
2. **DevWorkspaceTemplate** - Reusable components that can be referenced by multiple DevWorkspaces (plugins, parent devfiles)
3. **DevWorkspaceRouting** - Manages network routing and endpoints for workspace services
4. **DevWorkspaceOperatorConfig** - Cluster-wide and namespace-level operator configuration

**AI Agent Note**: When modifying workspace resources, understand which API is appropriate for your changes.

## Advanced Features

### Workspace Bootstrapping
Supported via `controller.devfile.io/bootstrap-devworkspace: true` attribute.
- Used when a devfile cannot be resolved but the project can be cloned.
- **Flow**:
    1. Workspace starts with generic deployment.
    2. `project-clone` container clones projects.
    3. `project-clone` finds devfile in cloned project.
    4. DevWorkspace is updated with the found devfile.
    5. Workspace restarts with new definition.

### Networking & Routing Classes
`DevWorkspaceRouting` resources use `.spec.routingClass` to determine how networking is handled.
- `basic`: Creates Route/Ingress (no auth).
- `cluster`: Creates Service only (internal access).
- `cluster-tls`: Creates Service + Serving Cert (OpenShift only).
- `web-terminal`: Alias for `cluster-tls`.
- Custom classes (e.g., `che`) are ignored by DWO and handled by external operators.

### Flexible Contributions
DevWorkspaces can import other Devfiles/Templates via `.spec.contributions`.
- Replaces legacy `plugin` components.
- Allows overriding fields (image, memory, env) of imported components.
- **Key Pattern**: Use for standardizing workspace definitions while allowing per-workspace customization.

## Quick Decision Guide

Use this as a quick reference for common decision points. When you encounter a scenario, find the matching "IF" condition and follow the "THEN" action.

### Error Handling Decisions

- **IF** error occurs in reconcile operation **THEN** use `checkDWError()` to handle it
- **IF** error is temporary/recoverable **THEN** return `&dwerrors.RetryError{}`
- **IF** error is permanent/unrecoverable **THEN** return `&dwerrors.FailError{}`
- **IF** operation succeeded with warnings **THEN** return `&dwerrors.WarningError{}`
- **IF** error comes from sync operation **THEN** use `dwerrors.WrapSyncError()`
- **IF** workspace fails **THEN** call `failWorkspace()` with appropriate `FailureReason`

### Client Usage Decisions

- **IF** you need fresh data (not cached) **THEN** use `NonCachingClient`
- **IF** reading resource immediately after creating/updating **THEN** use `NonCachingClient`
- **IF** external system may have modified resource **THEN** use `NonCachingClient`
- **IF** normal read operation **THEN** use regular `Client` (cached, more efficient)

### Metrics Decisions

- **IF** workspace starts **THEN** call `metrics.WorkspaceStarted()`
- **IF** workspace enters Running phase **THEN** call `metrics.WorkspaceRunning()`
- **IF** workspace fails **THEN** call `metrics.WorkspaceFailed()` with `FailureReason`
- **IF** determining failure reason **THEN** use `metrics.DetermineProvisioningFailureReason()` or `metrics.GetFailureReason()`

### Infrastructure Detection Decisions

- **IF** code behaves differently on OpenShift vs Kubernetes **THEN** check `infrastructure.IsOpenShift()`
- **IF** using infrastructure functions **THEN** ensure `infrastructure.Initialize()` was called first (in `init()` or early `main()`)
- **IF** in test code **THEN** use `infrastructure.InitializeForTesting()` to mock infrastructure type
- ⚠️ **WARNING**: Calling `infrastructure.IsOpenShift()` before `infrastructure.Initialize()` will **panic**

### Code Generation Decisions

- **IF** you modified API types (in `apis/` directory) **THEN** run `make generate_all`
- **IF** you added/removed kubebuilder markers **THEN** run `make generate_all`
- **IF** you changed CRD definitions **THEN** run `make generate_all`
- **IF** you modified struct fields in API types **THEN** run `make generate_all`

### Webhook Decisions

- **IF** creating mutating webhook **THEN** use `WebhookHandler` struct with `Decoder` and `Client`
- **IF** webhook needs to modify object **THEN** return `admission.Patched()`
- **IF** webhook needs to reject request **THEN** return `admission.Denied()`
- **IF** webhook has decode error **THEN** return `admission.Errored(http.StatusBadRequest, err)`

### RBAC Decisions

- **IF** controller needs to access resource **THEN** add `// +kubebuilder:rbac` marker above Reconcile function
- **IF** resource is in core API **THEN** use `groups=""`
- **IF** multiple resources in one marker **THEN** separate with semicolons: `resources=pods;services;configmaps`

### Status Update Decisions

- **IF** updating workspace status **THEN** use defer pattern with `updateWorkspaceStatus()`
- **IF** workspace phase changes **THEN** update both `status.phase` and conditions
- **IF** status must always update (even on early return) **THEN** use defer pattern

### Testing Decisions

- **IF** testing exported functions only **THEN** use `package controllers_test` (external)
- **IF** testing internal/private functions **THEN** use `package controllers` (internal)
- **IF** test needs async wait **THEN** use `Eventually()`, not `time.Sleep()`
- **IF** documenting test steps **THEN** use `By("step description")`
- **IF** e2e test creates DevWorkspace **THEN** MUST use `DeleteDevWorkspaceAndWait` in cleanup (AfterAll or AfterEach)
- **IF** e2e test suite is `ginkgo.Ordered` **THEN** use `AfterAll` for cleanup
- **IF** e2e test runs multiple times **THEN** use `AfterEach` for cleanup

### Workspace Bootstrapping Decisions

- **IF** workspace needs to run with limited devfile access **THEN** use workspace bootstrapping features
- **IF** workspace needs to import plugins/components flexibly **THEN** use DevWorkspaceTemplate references

## Project-Specific Conventions

### File Headers

All Go source files MUST start with this copyright header (replace `{CURRENT_YEAR}` with the current year):

```go
// Copyright (c) 2019-{CURRENT_YEAR} Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

```

### Import Organization

Three groups separated by blank lines: (1) standard library, (2) third-party + Kubernetes, (3) project-local.
Run `make fmt` to enforce this automatically.

```go
import (
  "context"
  "fmt"

  dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
  corev1 "k8s.io/api/core/v1"
  k8sErrors "k8s.io/apimachinery/pkg/api/errors"
  ctrl "sigs.k8s.io/controller-runtime"

  controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
  "github.com/devfile/devworkspace-operator/pkg/common"
  "github.com/devfile/devworkspace-operator/pkg/dwerrors"
)
```

**Common Import Aliases**:

- `dw` - DevWorkspace API types
- `corev1` - Kubernetes core API v1
- `k8sErrors` - Kubernetes API errors
- `ctrl` - controller-runtime
- `controllerv1alpha1` - Controller API types
- `wkspConfig` - Workspace configuration package

### File Formatting

**AI Agent Note**: ALWAYS ensure every file (except generated files) ends with a trailing newline.

All source files MUST end with a single trailing newline character (POSIX standard).

- ✅ **DO** ensure files end with `\n` (newline character)
- ✅ **DO** verify this when creating or editing files
- ❌ **DON'T** add trailing newlines to generated files (e.g., `zz_generated.*`, CRD manifests)
- ❌ **DON'T** add multiple trailing newlines (only one)

**Why**: Trailing newlines are part of the POSIX definition of a text file and help with:
- Version control diffs (Git shows "No newline at end of file" warnings without it)
- File concatenation and text processing tools
- Standard compliance and editor compatibility

**Note**: Most modern editors handle this automatically, but when using Write or Edit tools, always ensure the final character is `\n`.

### Naming Conventions

- **Packages**: lowercase, descriptive (`workspace`, `library`, `provision`)
- **Types**: PascalCase (`DevWorkspaceReconciler`, `RetryError`)
- **Functions**: camelCase for private, PascalCase for exported (`syncDeployment`, `SyncDeploymentToCluster`)
- **Variables**: camelCase (`workspaceID`, `namespace`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE

## Critical Patterns

### Error Handling with Custom Types

**AI Agent Note**: NEVER return raw errors from reconcile helpers. Always use custom error types and `checkDWError()`.

The project uses three custom error types in `pkg/dwerrors/`:

1. `RetryError` - temporary, will retry (can specify `RequeueAfter`)
2. `FailError` - permanent failure, workspace marked as Failed
3. `WarningError` - non-critical, workspace continues

**Pattern**: Return custom error types from reconcile helpers:

```go
// Temporary error (will retry)
if err != nil {
  return &dwerrors.RetryError{
    Err:          err,
    Message:      "failed to sync deployment",
    RequeueAfter: 5 * time.Second, // optional
  }
}

// Permanent failure
if !isValid {
  return &dwerrors.FailError{
    Message: "invalid devfile configuration",
  }
}

// Wrap sync errors (auto-converts to appropriate type)
if err := syncResource(); err != nil {
  return dwerrors.WrapSyncError(err)
}
```

**Pattern**: Use `checkDWError()` in reconcile to process errors:

```go
result, err := r.reconcileWorkspace(ctx, workspace, reconcileStatus)
if shouldReturn, res, returnErr := r.checkDWError(
  workspace, err, "Failed to reconcile",
  metrics.ReasonInfrastructureFailure, log, reconcileStatus,
); shouldReturn {
  return res, returnErr
}
```

**Decision Tree**:

```text
Is error temporary/recoverable?
├─ YES → RetryError
└─ NO → Is it permanent failure?
    ├─ YES → FailError
    └─ NO → Is it from sync operation?
        └─ YES → WrapSyncError()
```

### Status Updates with Defer

**AI Agent Note**: Always use defer for status updates to ensure they happen even on early returns.

**Pattern**: Use defer to guarantee status updates:

```go
func (r *DevWorkspaceReconciler) reconcileWorkspace(...) (reconcile.Result, error) {
  var reconcileResult reconcile.Result
  var reconcileErr error

  defer func() {
    reconcileResult, reconcileErr = r.updateWorkspaceStatus(
      workspace, log, status, reconcileResult, reconcileErr)
  }()

  // Reconciliation logic...
  // If function returns early, status still updates via defer

  return reconcileResult, reconcileErr
}
```

### Failing Workspaces

**Pattern**: When workspace fails, call `failWorkspace()` with appropriate `FailureReason`:

```go
return r.failWorkspace(
  workspace,
  "Error message",
  metrics.ReasonInfrastructureFailure, // or ReasonBadRequest, ReasonWorkspaceEngineFailure
  log,
  status,
)
```

**FailureReason values**:

- `ReasonBadRequest` - Invalid config, image pull errors, crash loops
- `ReasonInfrastructureFailure` - Scheduling failures, mount errors
- `ReasonWorkspaceEngineFailure` - Workspace engine/operator errors
- `ReasonUnknown` - Cannot be determined

### Infrastructure Detection Pattern

**AI Agent Note**: NEVER call `infrastructure.IsOpenShift()` without first calling `infrastructure.Initialize()`. This will panic.

**Pattern**: Initialize in `init()` or early `main()`:

```go
func init() {
  err := infrastructure.Initialize()
  if err != nil {
    setupLog.Error(err, "could not determine cluster type")
    os.Exit(1)
  }

  if infrastructure.IsOpenShift() {
    // OpenShift-specific initialization
  }
}
```

**Pattern**: Check infrastructure type for platform-specific code:

```go
if infrastructure.IsOpenShift() {
  // Use Routes, OpenShift SCC, etc.
} else {
  // Use Ingress, standard Kubernetes
}
```

### Client Usage (Cached vs Non-Cached)

**AI Agent Note**: Default to cached `Client`. Only use `NonCachingClient` when cache staleness is a problem.

**Performance Note**: Caching clients significantly improve performance by reducing API server calls. The controller-runtime cache automatically updates on resource changes. Default to cached client unless you have a specific reason for non-cached access.

**Pattern**: Use non-caching client when:

1. Reading immediately after create/update
2. External systems may have modified resource
3. Loading configuration that may be updated externally

```go
// Reading immediately after create
if err := r.Create(ctx, configMap); err != nil {
  return ctrl.Result{}, err
}
// Cache might not have it yet - use NonCachingClient
if err := r.NonCachingClient.Get(ctx, key, &createdConfigMap); err != nil {
  return ctrl.Result{}, err
}
```

### Webhook Pattern

**Pattern**: Mutating webhook modifies objects before persistence:

```go
func (h *WebhookHandler) MutateWorkspaceV1alpha2OnCreate(
  ctx context.Context, req admission.Request,
) admission.Response {
  wksp := &dwv2.DevWorkspace{}
  if err := h.Decoder.Decode(req, wksp); err != nil {
    return admission.Errored(http.StatusBadRequest, err)
  }

  // Modify object
  wksp.Labels[constants.DevWorkspaceCreatorLabel] = req.UserInfo.UID

  // Validate permissions
  if err := h.validateUserPermissions(ctx, req, wksp, nil); err != nil {
    return admission.Denied(err.Error())
  }

  return h.returnPatched(req, wksp)
}
```

**Pattern**: Validating webhook checks but doesn't modify:

```go
func (h *WebhookHandler) ValidateWorkspace(...) admission.Response {
  if err := validateWorkspaceSpec(wksp); err != nil {
    return admission.Denied(err.Error())
  }
  return admission.Allowed("validation passed")
}
```

### RBAC Markers

**Pattern**: Add kubebuilder RBAC markers above Reconcile function:

```go
// Core API resources (empty group)
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps,verbs=get;list;watch;create;update

// Custom resources (specify API group)
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workspace.devfile.io,resources=devworkspaces/status,verbs=get;update;patch

// Cluster-scoped resources
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update
func (r *DevWorkspaceReconciler) Reconcile(...) (ctrl.Result, error) {
```

### Testing Pattern

**AI Agent Note**: Most controller tests use `package controllers_test` (external). Check existing test files for pattern.

**Pattern**: External test package (default):

```go
package controllers_test

import (
  . "github.com/onsi/ginkgo/v2"
  . "github.com/onsi/gomega"
)

var _ = Describe("DevWorkspace Controller", func() {
  It("Sets DevWorkspace ID and Starting status", func() {
    By("Creating a new DevWorkspace")
    // Create resource

    By("Checking DevWorkspace ID has been set")
    Eventually(func() (string, error) {
      // Get and return value
    }, timeout, interval).Should(Equal(expectedValue))
  })
})
```

### E2E Test Cleanup Pattern

**AI Agent Note**: ALWAYS add PVC cleanup to e2e tests to prevent conflicts between test runs, especially in CI environments.

**Critical**: DevWorkspaces use a shared PVC (`claim-devworkspace`) that persists after workspace deletion. Without proper cleanup, subsequent tests can fail due to PVC conflicts or stale data.

**Pattern**: Use `DeleteDevWorkspaceAndWait` in cleanup blocks:

```go
var _ = ginkgo.Describe("[Test Suite Name]", ginkgo.Ordered, func() {
  defer ginkgo.GinkgoRecover()

  const workspaceName = "test-workspace"

  ginkgo.AfterAll(func() {
    // Cleanup workspace and wait for PVC to be fully deleted
    // This prevents PVC conflicts in subsequent tests, especially in CI environments
    _ = config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)
  })

  ginkgo.It("Test case", func() {
    // Test implementation
  })
})
```

**Decision Tree for Cleanup**:
- **IF** test suite runs multiple times with different workspaces **THEN** use `AfterEach`
- **IF** test suite uses `ginkgo.Ordered` (sequential tests on same workspace) **THEN** use `AfterAll`
- **IF** test creates workspace **THEN** MUST include cleanup with `DeleteDevWorkspaceAndWait`

**Available Helper Functions** (in `test/e2e/pkg/client/devws.go`):
- `DeleteDevWorkspace(name, namespace)` - Deletes workspace only (fast, may leave PVC)
- `WaitForPVCDeleted(pvcName, namespace, timeout)` - Waits for PVC deletion
- `DeleteDevWorkspaceAndWait(name, namespace)` - Deletes workspace and waits for PVC cleanup (RECOMMENDED)

**Example: AfterEach Pattern** (for tests running multiple times):

```go
ginkgo.Context("Test context", func() {
  const workspaceName = "test-workspace"

  ginkgo.BeforeEach(func() {
    // Setup
  })

  ginkgo.It("Test case", func() {
    // Test implementation
  })

  ginkgo.AfterEach(func() {
    // Cleanup workspace and wait for PVC to be fully deleted
    // This prevents PVC conflicts in subsequent tests, especially in CI environments
    _ = config.DevK8sClient.DeleteDevWorkspaceAndWait(workspaceName, config.DevWorkspaceNamespace)
  })
})
```

**Why This Matters**:
- **CI Flakiness**: Without PVC cleanup, tests can fail intermittently in CI with "PVC already exists" errors
- **Stale Data**: Old PVC data can affect test results and cause false positives/negatives
- **Cloud Environments**: PVC deletion can be slow (30-60+ seconds), requiring explicit wait
- **Test Isolation**: Each test should start with a clean state

### Deep Copy Pattern

**AI Agent Note**: Always DeepCopy objects from cache before modifying to avoid race conditions.

**Pattern**:

```go
workspace := &dw.DevWorkspace{}
if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
  return ctrl.Result{}, err
}

// DeepCopy before modifying
workspaceCopy := workspace.DeepCopy()
workspaceCopy.Labels["new-label"] = "value"

if err := r.Update(ctx, workspaceCopy); err != nil {
  return ctrl.Result{}, err
}
```

### Reconciliation Best Practices

**AI Agent Note**: Reconciliation must be fast and focused. Don't save state between reconciles.

**Pattern**: Keep reconciliation fast and stateless:

- ✅ **DO** make reconciliation idempotent - running multiple times produces same result
- ✅ **DO** keep reconciliation fast - avoid long-running operations in reconcile loop
- ✅ **DO** move cluster state toward expected configuration incrementally
- ❌ **DON'T** save state between reconciles - always read current state from cluster
- ❌ **DON'T** perform blocking operations - use requeue for async work

**Reconciliation Philosophy**: Reconciliation is about moving cluster state towards an expected configuration, not executing a series of steps. Each reconcile should assess current state and take the next action needed.

## Directory Structure

```text
├── apis/                    # API type definitions and CRD schemas
│                            # Modify when: Adding/changing API types, CRDs
├── controllers/             # Controller implementations
│   ├── workspace/          # Main workspace reconciliation
│   ├── controller/devworkspacerouting/  # Routing controllers
│   └── cleanupcronjob/    # Cleanup controllers
│                            # Modify when: Adding controllers, changing reconciliation
├── pkg/                    # Shared library code
│   ├── common/            # Common types and utilities
│   ├── library/           # Reusable libraries
│   ├── provision/         # Resource provisioning logic
│   ├── dwerrors/          # Custom error types (RetryError, FailError, WarningError)
│   ├── config/            # Configuration management
│   ├── infrastructure/    # Infrastructure detection (OpenShift vs Kubernetes)
│   └── conditions/        # Condition management
│                            # Modify when: Adding shared utilities, common patterns
├── webhook/               # Admission webhook implementations
│                            # Modify when: Adding webhooks, changing validation/mutation
├── deploy/                # Deployment manifests and templates
│                            # Modify when: Changing deployment configs, OLM manifests
├── test/                  # E2E and integration tests
│                            # Modify when: Adding test cases
└── samples/               # Sample DevWorkspace definitions
                            # Modify when: Adding example workspaces
```

## Common Pitfalls (Don'ts)

### Code Quality

- ❌ Don't hardcode namespaces, image names, or URLs
- ❌ Don't use `panic()` in production code
- ❌ Don't ignore errors (always handle or propagate)
- ❌ Don't log sensitive information (secrets, tokens)
- ❌ Don't use `time.Sleep()` in reconciliation (use Requeue instead)
- ❌ Don't modify objects from cache directly (always DeepCopy first)
- ❌ Don't perform long-running operations in reconcile - keep reconciliation fast
- ❌ Don't save state between reconciles - always read from cluster
- ❌ Don't forget to add trailing newline at end of files (except generated files)

### Error Handling

- ❌ Don't return raw errors from reconcile helpers (use custom error types)
- ❌ Don't skip `checkDWError()` for reconcile operation errors
- ❌ Don't forget to record metrics when workspace state changes

### Infrastructure & Configuration

- ❌ Don't call `infrastructure.IsOpenShift()` before `infrastructure.Initialize()` (will panic)
- ❌ Don't use cached client when you need fresh data (use `NonCachingClient`)
- ❌ Don't forget to call `infrastructure.Initialize()` in `init()` or early `main()`

### Code Generation

- ❌ Don't forget to run `make generate_all` after modifying APIs or kubebuilder markers
- ❌ Don't commit API changes without regenerating code
- ❌ Don't modify generated files (zz_generated.*, CRD manifests) manually

### API Changes

- ❌ Don't break backward compatibility of CRD APIs
- ❌ Don't remove/rename existing API fields without deprecation
- ❌ Don't change default values of existing fields
- ❌ Don't modify CRDs without careful planning - breaking changes affect all users
- ❌ Don't add required fields to existing CRDs - this breaks existing resources
- ❌ Don't change field types in CRDs - use new fields with deprecation instead

### Testing

- ❌ Don't commit disabled tests without tracking issues
- ❌ Don't write tests that depend on timing (use Eventually/Consistently)
- ❌ Don't leave test resources unmanaged (always clean up)
- ❌ Don't forget PVC cleanup in e2e tests (use `DeleteDevWorkspaceAndWait`)
- ❌ Don't use `DeleteDevWorkspace` alone in e2e tests (PVC may persist and cause conflicts)

## Debugging

### Log Format and Workspace Identification

**AI Agent Note**: DWO uses JSON-formatted logs. Always filter by workspace ID or namespace.

**Pattern**: Logs include `devworkspace_id`, `Request.Namespace`, and `Request.Name`.
```bash
# Filter logs for specific workspace by name
kubectl logs -l app.kubernetes.io/name=devworkspace-controller -n devworkspace-controller | \
  jq 'select(.workspace.name == "my-workspace")'

# Filter by ID
kubectl logs -l app.kubernetes.io/name=devworkspace-controller -n devworkspace-controller | \
  jq 'select(.workspace.id == "workspace-id-12345")'
```

**Reconcile Tracing**:
- Start: `msg: Reconciling Workspace` (includes `resolvedConfig`)
- End: A log line indicating action (e.g., `updated object`, `workspace running`)

### Experimental Diff Logging

**Feature**: DWO can log diffs of applied changes.
**Enable**: Set `enableExperimentalFeatures: true` in `DevWorkspaceOperatorConfig`.
**Warning**: May log sensitive data (Secrets) in cleartext. Use only for debugging.
**Usage**: Extract and decode the diff from logs programmatically:
```bash
kubectl logs ... | grep "Diff:" | jq -r .msg | xargs -0 echo -e
```

### Local Debugging

**Pattern**: Run controller locally while connected to a remote cluster.
1. Check out the matching DWO version locally.
2. Scale down the cluster deployment:
   ```bash
   kubectl scale deploy devworkspace-controller-manager -n devworkspace-controller --replicas=0
   ```
3. Run locally:
   ```bash
   make run
   ```

### Webhook Debugging

**Critical Note**: `restricted-access` workspaces (and Web Terminals) route `pods/exec` through the webhook server.
- **Symptom**: `oc exec` or terminal access fails globally.
- **Check**: Verify `devworkspace-webhook-server` deployment is running.
- **Impact**: If webhook server is down, *all* exec requests in the cluster may fail if they match the webhook selector (which includes `devworkspace_id`).

### Reproducing Issues

**Pattern**: Copy workspace resources to reproduce issues:
1. Export the failing DevWorkspace: `kubectl get devworkspace <name> -o yaml > workspace.yaml`
2. Export related DevWorkspaceTemplates.
3. Clean up status and metadata:
   ```bash
   yq -i 'del(.metadata.creationTimestamp, .metadata.generation, .metadata.resourceVersion, .metadata.uid, .metadata.annotations."kubectl.kubernetes.io/last-applied-configuration", .status)' workspace.yaml
   ```
4. Apply to a fresh namespace.

## Build & Development

**AI Agent Note**: Always run `make test` before committing. Run `make generate_all` after modifying APIs or kubebuilder markers.

### Common Make Targets

```bash
make test             # Run unit tests
make generate_all     # Generate code, CRDs, manifests (REQUIRED after API changes)
make fmt              # Format code with goimports
make vet              # Run go vet
make docker           # Build and push controller image
make install          # Install CRDs and deploy controller
make run              # Run controller locally
```

### Development Workflow

1. Make code changes
2. Run `make generate_all` if you modified APIs or kubebuilder markers
3. Run `make test` to verify tests pass
4. Run `make fmt` and `make vet` for linting
5. Commit with sign-off: `git commit -s -m "message"`

### Code Generation Triggers

Run `make generate_all` when:

- Modified types in `apis/` directory
- Added/removed kubebuilder markers (`// +kubebuilder:...`)
- Changed CRD definitions
- Modified struct fields in API types
- Added/removed RBAC markers

### Environment Variables

- `DWO_IMG`: Controller image (default: `quay.io/devfile/devworkspace-controller:next`)
- `NAMESPACE`: Deployment namespace (default: `devworkspace-controller`)
- `DOCKER`: Container tool (`docker` or `podman`, auto-detected)

## Version Information

**AI Agent Note**: ALWAYS check `go.mod` for current versions before suggesting code. Do not hallucinate versions.

- **Go Version**: Strictly from `go.mod`
- **Kubernetes API**: Strictly from `go.mod`
- **controller-runtime**: Strictly from `go.mod`
- **Devfile API**: Strictly from `go.mod`

**Important**: Always verify versions by reading `go.mod` directly, as dependencies are regularly updated.

## Common Operations Reference

**AI Agent Note**: Use these direct commands to manage DevWorkspace state during debugging or testing.

### Get Controller Logs
```bash
kubectl logs -n ${NAMESPACE:-devworkspace-controller} deploy/devworkspace-controller-manager -c devworkspace-controller
```

### Stop a Workspace
```bash
kubectl patch dw <name> --type merge -p '{"spec": {"started": false}}'
```

### Start a Workspace
```bash
kubectl patch dw <name> --type merge -p '{"spec": {"started": true}}'
```

### Force Reconcile (Touch)
Use this to trigger a reconciliation without changing the spec.
```bash
kubectl patch dw <name> --type merge -p "{\"metadata\": {\"annotations\": {\"force-update\": \"$(date +%s)\"}}}"
```

## See Also

- **[README.md](README.md)** - Project overview, installation, what is DevWorkspace Operator
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Development setup, debugging, testing, contribution guidelines
- **[redhat-compliance-and-responsible-ai.md](redhat-compliance-and-responsible-ai.md)** - Red Hat compliance & responsible AI rules
- **[docs/additional-configuration.adoc](docs/additional-configuration.adoc)** - DevWorkspace configuration options
- **[docs/unsupported-devfile-api.adoc](docs/unsupported-devfile-api.adoc)** - Unsupported Devfile API features
- **[Devfile Documentation](https://devfile.io/)** - Devfile specification
- **[Kubebuilder Book](https://book.kubebuilder.io/)** - Controller patterns and best practices

---

**Last Updated**: 2025-12-24
