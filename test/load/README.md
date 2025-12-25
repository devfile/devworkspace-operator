# Load Testing for DevWorkspace Operator

This directory contains load testing tools for the DevWorkspace Operator using k6. The tests create multiple DevWorkspaces concurrently to measure the operator's performance under load.

## Prerequisites

- `kubectl` (version >= 1.24.0)
- `curl` (version >= 7.0.0)
- `k6` (version >= 1.1.0) - Required when using `--mode binary`
- Access to a Kubernetes cluster with DevWorkspace Operator installed
- Proper RBAC permissions to create DevWorkspaces, ConfigMaps, Secrets, and Namespaces

## Running Load Tests

The load tests can be run using the `make test_load` target with various arguments. The tests support two modes:
- **binary mode**: Runs k6 locally (default)
- **operator mode**: Runs k6 using the k6-operator in the cluster

### Running with Eclipse Che

When running with Eclipse Che, the script automatically provisions additional ConfigMaps for certificates that are required for Che workspaces to function properly.

```bash
make test_load ARGS=" \
  --mode binary \
  --run-with-eclipse-che true \
  --max-vus ${MAX_VUS} \
  --create-automount-resources true \
  --max-devworkspaces ${MAX_DEVWORKSPACES} \
  --devworkspace-ready-timeout-seconds 3600 \
  --delete-devworkspace-after-ready false \
  --separate-namespaces false \
  --test-duration-minutes 40"
```

**Note**: When `--run-with-eclipse-che true` is set, the script will:
- Provision a workspace namespace compatible with Eclipse Che
- Create additional certificate ConfigMaps required by Che

### Running without Eclipse Che

When running without Eclipse Che, the standard namespace setup is used without additional certificate ConfigMaps.

```bash
make test_load ARGS=" \
  --mode binary \
  --max-vus ${MAX_VUS} \
  --create-automount-resources true \
  --max-devworkspaces ${MAX_DEVWORKSPACES} \
  --devworkspace-ready-timeout-seconds 3600 \
  --delete-devworkspace-after-ready false \
  --separate-namespaces false \
  --test-duration-minutes 40"
```

## Available Parameters

| Parameter | Description | Default | Example |
|-----------|-------------|---------|---------|
| `--mode` | Execution mode: `binary` or `operator` | `binary` | `--mode binary` |
| `--max-vus` | Maximum number of virtual users (concurrent DevWorkspace creations) | `100` | `--max-vus 50` |
| `--max-devworkspaces` | Maximum number of DevWorkspaces to create (-1 for unlimited) | `-1` | `--max-devworkspaces 200` |
| `--separate-namespaces` | Create each DevWorkspace in its own namespace | `false` | `--separate-namespaces true` |
| `--delete-devworkspace-after-ready` | Delete DevWorkspace once it becomes Ready | `true` | `--delete-devworkspace-after-ready false` |
| `--devworkspace-ready-timeout-seconds` | Timeout in seconds for workspace to become ready | `1200` | `--devworkspace-ready-timeout-seconds 3600` |
| `--devworkspace-link` | URL to external DevWorkspace JSON to use instead of default | (empty) | `--devworkspace-link https://...` |
| `--create-automount-resources` | Create automount ConfigMap and Secret for testing | `false` | `--create-automount-resources true` |
| `--dwo-namespace` | DevWorkspace Operator namespace | `openshift-operators` | `--dwo-namespace devworkspace-controller` |
| `--logs-dir` | Directory for DevWorkspace and event logs | `logs` | `--logs-dir /tmp/test-logs` |
| `--test-duration-minutes` | Duration in minutes for the load test | `25` | `--test-duration-minutes 40` |
| `--run-with-eclipse-che` | Enable Eclipse Che integration (adds certificate ConfigMaps) | `false` | `--run-with-eclipse-che true` |
| `--che-cluster-name` | Eclipse Che cluster name (when using Che) | `eclipse-che` | `--che-cluster-name my-che` |
| `--che-namespace` | Eclipse Che namespace (when using Che) | `eclipse-che` | `--che-namespace my-che-ns` |

## What the Tests Do

1. **Setup**: Creates a test namespace, ServiceAccount, and RBAC resources
2. **Eclipse Che Setup** (if enabled): Provisions Che-compatible namespace and certificate ConfigMaps
3. **Load Generation**: Creates DevWorkspaces concurrently based on `--max-devworkspaces`
4. **Monitoring**: 
   - Watches DevWorkspace status until Ready
   - Monitors operator CPU and memory usage
   - Tracks etcd metrics
   - Logs events and DevWorkspace state changes
5. **Cleanup**: Removes all created resources and test namespace

## Test Metrics

The tests track the following metrics:
- DevWorkspace creation duration
- DevWorkspace ready duration
- DevWorkspace deletion duration
- Operator CPU and memory usage
- etcd CPU and memory usage
- Success/failure rates

## Output

- **Logs**: Stored in the `logs/` directory (or custom directory specified by `--logs-dir`)
  - `{timestamp}_events.log`: Kubernetes events
  - `{timestamp}_dw_watch.log`: DevWorkspace watch logs
  - `dw_failure_report.csv`: Failed DevWorkspaces report
- **HTML Report**: Generated when running in binary mode (outside cluster)
- **Console Output**: Real-time test progress and summary

## Troubleshooting

- **Permission errors**: Ensure your kubeconfig has sufficient RBAC permissions
- **Timeout errors**: Increase `--devworkspace-ready-timeout-seconds` for slower clusters
- **Resource exhaustion**: Reduce `--max-vus` or `--max-devworkspaces` if cluster resources are limited
- **k6 not found**: Install k6 from https://k6.io/docs/getting-started/installation/

## Additional Notes

- The tests use an opinionated minimal DevWorkspace by default, or you can provide a custom one via `--devworkspace-link`
- When `--separate-namespaces true` is used, each DevWorkspace gets its own namespace
- The `--delete-devworkspace-after-ready false` option is useful for testing sustained load scenarios
- Certificate ConfigMaps are only created when `--run-with-eclipse-che true` is set

