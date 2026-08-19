# TLS Adherence Feature Test Plan

Tests that DevWorkspace Operator honors the cluster TLS profile when `tlsAdherence: StrictAllComponents` is set.

## Prerequisites

```bash
# Verify OpenShift cluster and DWO installation
oc get deployment -n openshift-operators devworkspace-controller-manager
oc get deployment -n openshift-operators devworkspace-webhook-server
```

## Test 1: Default Behavior (No Adherence Policy)

By default, `tlsAdherence` is not set and DWO uses Go's default TLS config.

```bash
# Check current policy (should be empty)
oc get apiserver cluster -o jsonpath='{.spec.tlsAdherence}{"\n"}'

# Check controller logs
CONTROLLER_POD=$(oc get pods -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-operators $CONTROLLER_POD -c devworkspace-controller | grep -i "tls"
```

**Expected**: Log shows `"using Go default TLS configuration"` with empty or no policy.

## Test 2: Enable StrictAllComponents

Enable strict adherence and verify DWO applies the cluster TLS profile.

```bash
# Set StrictAllComponents with a TLS profile
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Intermediate","intermediate":{}},"tlsAdherence":"StrictAllComponents"}}'

# Delete controller pod to pick up new policy
oc delete pod -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller

# Wait for new pod
sleep 10

# Check logs
CONTROLLER_POD=$(oc get pods -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-operators $CONTROLLER_POD -c devworkspace-controller | grep -A3 "Applying cluster TLS profile"
```

**Expected**: Log shows:
```
"Applying cluster TLS profile to metrics and webhook servers"
  minTLSVersion="VersionTLS12"
  adherencePolicy="StrictAllComponents"
```

## Test 3: Profile Change Detection

Verify controller restarts when TLS profile changes.

```bash
# Change to a different profile (e.g., Modern)
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsSecurityProfile":{"type":"Modern","modern":{}}}}'

# Wait for automatic restart
sleep 20

# Get new pod and check logs
CONTROLLER_POD=$(oc get pods -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-operators $CONTROLLER_POD -c devworkspace-controller | grep -A3 "Applying cluster TLS profile"
```

**Expected**: Log shows `minTLSVersion="VersionTLS13"` (Modern profile) and a restart message like `"TLS security profile changed; initiating graceful restart"`.

## Test 4: Policy Change Detection

Verify controller restarts when adherence policy changes.

```bash
# Change policy to LegacyAdheringComponentsOnly (does not honor profile)
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsAdherence":"LegacyAdheringComponentsOnly"}}'

# Wait for automatic restart
sleep 20

# Check logs
CONTROLLER_POD=$(oc get pods -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-operators $CONTROLLER_POD -c devworkspace-controller | grep -i "tls"
```

**Expected**: Log shows `"using Go default TLS configuration"` with `policy="LegacyAdheringComponentsOnly"` and a restart message like `"TLS adherence policy changed; initiating graceful restart"`.

## Test 5: Smoke Test

Verify controller functions correctly with TLS adherence enabled.

```bash
# Re-enable StrictAllComponents
oc patch apiserver cluster --type=merge -p '{"spec":{"tlsAdherence":"StrictAllComponents"}}'

# Wait for automatic restart
sleep 20

# Create test workspace
cat <<EOF | oc apply -f -
apiVersion: workspace.devfile.io/v1alpha2
kind: DevWorkspace
metadata:
  name: tls-test-workspace
  namespace: default
spec:
  started: true
  template:
    components:
      - name: tooling
        container:
          image: quay.io/devfile/universal-developer-image:ubi8-latest
EOF

# Wait for workspace to start
oc get devworkspace tls-test-workspace -n default -w

# Clean up
oc delete devworkspace tls-test-workspace -n default
```

**Expected**: Workspace reaches `Running` phase.

## Cleanup

```bash
# Remove tlsAdherence and tlsSecurityProfile
oc patch apiserver cluster --type=json -p '[{"op":"remove","path":"/spec/tlsAdherence"},{"op":"remove","path":"/spec/tlsSecurityProfile"}]'

# Delete controller pod to reset to defaults
oc delete pod -n openshift-operators -l app.kubernetes.io/name=devworkspace-controller
```

## Notes

- **OLM-managed deployment**: DWO is managed by OLM (Operator Lifecycle Manager), so use `oc delete pod` instead of `oc rollout restart` to force a restart.
- **Automatic restarts**: Tests 3 and 4 verify the controller automatically restarts when the TLS profile or adherence policy changes (no manual restart needed).
