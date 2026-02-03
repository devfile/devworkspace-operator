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
- **DWO**: Waits for `deployment condition=available` (5-minute timeout)
- **Che**: Waits for `CheCluster condition=Available` (10-minute timeout)
- **Pods**: Verifies all Che pods are ready

### Artifact Collection
On each failure, collects:
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

### DWO Deployment Fails
**Symptoms**: "ERROR: DWO controller is not ready"
**Check**: `$ARTIFACT_DIR/devworkspace-controller-info/`
**Common causes**: Image pull errors, resource constraints, webhook conflicts

### Che Deployment Timeout
**Symptoms**: "ERROR: CheCluster did not become available within 10 minutes"
**Check**: `$ARTIFACT_DIR/che-operator-logs-attempt-*.log`
**Common causes**: Database connection issues, image pull failures, operator reconciliation errors

### Pod CrashLoopBackOff
**Symptoms**: "ERROR: chectl server:deploy failed"
**Check**: `$ARTIFACT_DIR/eclipse-che-info/` for pod logs
**Common causes**: Configuration errors, resource limits, TLS certificate issues

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
├── chectl-logs-attempt-1/
└── chectl-logs-attempt-2/
```

## Dependencies

- `kubectl` - Kubernetes CLI
- `oc` - OpenShift CLI (for log collection)
- `chectl` - Eclipse Che CLI (v7.114.0+)
- `jq` - JSON processor (for chectl)

## Related Documentation

- [Eclipse Che Documentation](https://eclipse.dev/che/docs/)
- [chectl GitHub Repository](https://github.com/che-incubator/chectl)
- [DevWorkspace Operator README](../README.md)
- [Contributing Guidelines](../CONTRIBUTING.md)
