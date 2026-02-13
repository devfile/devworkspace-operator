# Che Happy-Path Test

**Script**: `.ci/oci-devworkspace-happy-path.sh`
**Purpose**: Integration test validating DevWorkspace Operator with Eclipse Che deployment

## Overview

This script deploys and validates the full DevWorkspace Operator + Eclipse Che stack on OpenShift, ensuring the happy-path user workflow succeeds. It's used in the `v14-che-happy-path` Prow CI test.

## Features

### Retry Logic
- **Max retries**: 2 (3 total attempts)
- **Exponential backoff**: 60s base delay with 0-15s jitter
- **Cleanup**: Deletes failed Che deployment before retry

### Health Checks
- **OLM**: Verifies `catalog-operator` and `olm-operator` are available before Che deployment (2-minute timeout each)
- **DWO**: Waits for `deployment condition=available` (5-minute timeout)
- **Che**: Waits for `CheCluster condition=Available` (10-minute timeout)
- **Pods**: Verifies all Che pods are ready

### Artifact Collection
On each failure, collects:
- OLM diagnostics (Subscription, InstallPlan, CSV, CatalogSource)
- CatalogSource pod logs
- Che operator logs (last 1000 lines)
- CheCluster CR status (full YAML)
- All pod logs from Che namespace
- Kubernetes events
- chectl server logs

### Error Handling
- Graceful error handling with stage-specific messages
- Progress indicators: "Attempt 1/2", "Retrying in 71s..."
- No crash on failures

## Configuration

Environment variables (all optional):

| Variable | Default | Description |
|----------|---------|-------------|
| `CHE_NAMESPACE` | `eclipse-che` | Namespace for Che deployment |
| `MAX_RETRIES` | `2` | Maximum retry attempts |
| `BASE_DELAY` | `60` | Base delay in seconds for exponential backoff |
| `MAX_JITTER` | `15` | Maximum jitter in seconds |
| `ARTIFACT_DIR` | `/tmp/dwo-e2e-artifacts` | Directory for diagnostic artifacts |
| `DEVWORKSPACE_OPERATOR` | (required) | DWO image to deploy |

## Usage

### In Prow CI

The script is called automatically by the `v14-che-happy-path` Prow job. Prow sets `DEVWORKSPACE_OPERATOR` based on the context:

**For PR checks** (testing PR code):
```bash
export DEVWORKSPACE_OPERATOR="quay.io/devfile/devworkspace-controller:pr-${PR_NUMBER}-${COMMIT_SHA}"
./.ci/oci-devworkspace-happy-path.sh
```

**For periodic/nightly runs** (testing main branch):
```bash
export DEVWORKSPACE_OPERATOR="quay.io/devfile/devworkspace-controller:next"
./.ci/oci-devworkspace-happy-path.sh
```

### Local Testing
```bash
export DEVWORKSPACE_OPERATOR="quay.io/youruser/devworkspace-controller:your-tag"
export ARTIFACT_DIR="/tmp/my-test-artifacts"
./.ci/oci-devworkspace-happy-path.sh
```

## Test Flow

1. **Deploy DWO**
   - Runs `make install`
   - Waits for controller deployment to be available
   - Collects artifacts if deployment fails

2. **Deploy Che** (with retry)
   - Runs `chectl server:deploy` with extended timeouts (24h)
   - Waits for CheCluster condition=Available
   - Verifies all pods are ready
   - Collects artifacts on failure
   - Cleans up and retries if needed

3. **Run Happy-Path Test**
   - Downloads test script from Eclipse Che repository
   - Executes Che happy-path workflow
   - Collects artifacts on failure

## Exit Codes

- `0`: Success - All stages completed
- `1`: Failure - Check `$ARTIFACT_DIR` for diagnostics

## Timeouts

| Component | Timeout | Purpose |
|-----------|---------|---------|
| DWO deployment | 5 minutes | Pod becomes available |
| CheCluster Available | 10 minutes | Che fully deployed |
| Che pods ready | 5 minutes | All pods running |
| chectl pod wait/ready | 24 hours | Generous for slow environments |

## Common Failures

### OLM Infrastructure Not Ready
**Symptoms**: "ERROR: OLM infrastructure is not healthy, cannot proceed with Che deployment"
**Check**: `$ARTIFACT_DIR/olm-diagnostics-olm-check.yaml`
**Common causes**:
- OLM operators not running (`catalog-operator`, `olm-operator`)
- Cluster provisioning issues during bootstrap
- Resource constraints preventing OLM operator scheduling
**Resolution**: This indicates a fundamental cluster infrastructure issue. Check cluster health and OLM operator logs before retrying.

### DWO Deployment Fails
**Symptoms**: "ERROR: DWO controller is not ready"
**Check**: `$ARTIFACT_DIR/devworkspace-controller-info/`
**Common causes**: Image pull errors, resource constraints, webhook conflicts

### Che Deployment Timeout
**Symptoms**: "ERROR: CheCluster did not become available within 10 minutes"
**Check**: `$ARTIFACT_DIR/che-operator-logs-attempt-*.log`, `$ARTIFACT_DIR/olm-diagnostics-attempt-*.yaml`
**Common causes**:
- OLM subscription timeout (check `olm-diagnostics` for subscription state)
- Database connection issues
- Image pull failures
- Operator reconciliation errors

### Pod CrashLoopBackOff
**Symptoms**: "ERROR: chectl server:deploy failed"
**Check**: `$ARTIFACT_DIR/eclipse-che-info/` for pod logs
**Common causes**: Configuration errors, resource limits, TLS certificate issues

### OLM Subscription Stuck
**Symptoms**: Subscription timeout after 120 seconds with no resources created
**Check**: `$ARTIFACT_DIR/olm-diagnostics-attempt-*.yaml`, `$ARTIFACT_DIR/catalogsource-logs-attempt-*.log`
**Common causes**:
- CatalogSource pod not pulling/running
- InstallPlan not created (subscription cannot resolve dependencies)
- Cluster resource exhaustion preventing operator pod scheduling
**Resolution**: Check OLM operator logs and CatalogSource pod status. See "Advanced Troubleshooting" section for monitoring and alternative deployment options.

## Artifact Locations

After a failed test run:
```
$ARTIFACT_DIR/
├── devworkspace-controller-info/
│   ├── <pod-name>-<container>.log
│   └── events.log
├── eclipse-che-info/
│   ├── <pod-name>-<container>.log
│   └── events.log
├── che-operator-logs-attempt-1.log
├── che-operator-logs-attempt-2.log
├── checluster-status-attempt-1.yaml
├── checluster-status-attempt-2.yaml
├── olm-diagnostics-attempt-1.yaml
├── olm-diagnostics-attempt-2.yaml
├── catalogsource-logs-attempt-1.log
├── catalogsource-logs-attempt-2.log
├── chectl-logs-attempt-1/
└── chectl-logs-attempt-2/
```

## Dependencies

- `kubectl` - Kubernetes CLI
- `oc` - OpenShift CLI (for log collection)
- `chectl` - Eclipse Che CLI (v7.114.0+)
- `jq` - JSON processor (for chectl)

## Advanced Troubleshooting

### OLM Infrastructure Issues

If you experience persistent OLM subscription timeouts (see `olm-diagnostics-*.yaml` artifacts):

#### Option 1: OLM Health Check (Implemented)
The script now verifies OLM infrastructure health before deploying Che:
- Checks `catalog-operator` is available
- Checks `olm-operator` is available
- Verifies `openshift-marketplace` is accessible

If OLM is unhealthy, the test fails fast with diagnostic artifacts instead of waiting through timeouts.

#### Option 2: Monitor Subscription Progress (Advanced)
For debugging stuck subscriptions, you can add active monitoring to detect zero-progress scenarios earlier:

```bash
# Example: Monitor subscription state every 10 seconds
while [ $elapsed -lt 300 ]; do
  state=$(kubectl get subscription eclipse-che -n eclipse-che \
    -o jsonpath='{.status.state}' 2>/dev/null)
  echo "[$elapsed/300s] Subscription state: ${state:-unknown}"
  if [ "$state" = "AtLatestKnown" ]; then
    break
  fi
  sleep 10
  elapsed=$((elapsed + 10))
done
```

This helps identify whether subscriptions are progressing slowly vs. completely stuck.

#### Option 3: Skip OLM Installation (Alternative Approach)
For CI environments with persistent OLM issues, consider deploying Che operator directly instead of via OLM:

```bash
chectl server:deploy \
  --installer=operator \  # Uses direct YAML deployment
  -p openshift \
  --batch \
  --telemetry=off \
  --skip-devworkspace-operator \
  --chenamespace="$CHE_NAMESPACE"
```

**Trade-offs**:
- ✅ Bypasses OLM infrastructure entirely
- ✅ More reliable in resource-constrained CI environments
- ❌ Doesn't test OLM integration path (used by production OperatorHub)
- ❌ May miss OLM-specific issues

**When to use**: Temporary workaround for CI infrastructure issues while OLM problems are being resolved.

### Subscription Timeout Issues

If OLM subscriptions consistently timeout (visible in `olm-diagnostics-*.yaml`):

1. **Check OLM operator logs**:
   ```bash
   kubectl logs -n openshift-operator-lifecycle-manager \
     deployment/catalog-operator --tail=100
   kubectl logs -n openshift-operator-lifecycle-manager \
     deployment/olm-operator --tail=100
   ```

2. **Verify CatalogSource pod is running**:
   ```bash
   kubectl get pods -n openshift-marketplace \
     -l olm.catalogSource=eclipse-che
   kubectl logs -n openshift-marketplace \
     -l olm.catalogSource=eclipse-che
   ```

3. **Check InstallPlan creation**:
   ```bash
   kubectl get installplan -n eclipse-che -o yaml
   ```
   - If no InstallPlan exists, OLM couldn't resolve the subscription
   - If InstallPlan exists but isn't complete, check its status conditions

## Related Documentation

- [Eclipse Che Documentation](https://eclipse.dev/che/docs/)
- [chectl GitHub Repository](https://github.com/che-incubator/chectl)
- [OLM Troubleshooting Guide](https://olm.operatorframework.io/docs/troubleshooting/)
- [DevWorkspace Operator README](../README.md)
- [Contributing Guidelines](../CONTRIBUTING.md)
