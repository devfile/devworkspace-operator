#!/bin/bash

#!/bin/bash

set -euo pipefail

MODE="binary"  # or 'operator'
NAMESPACE="loadtest-devworkspaces"
DWO_NAMESPACE="openshift-operators"
SA_NAME="k6-devworkspace-tester"
CLUSTERROLE_NAME="k6-devworkspace-role"
ROLEBINDING_NAME="k6-devworkspace-binding"
CONFIGMAP_NAME="k6-test-script"
K6_CR_NAME="k6-test-run"
K6_SCRIPT="test/load/devworkspace_load_test.js"
K6_OPERATOR_VERSION="v0.0.22"
DEVWORKSPACE_LINK="https://gist.githubusercontent.com/rohanKanojia/71fe35304009f036b6f6b8a8420fb67c/raw/c98c91c03cad77f759277104b860ce3ca52bf6c2/simple-ephemeral.json"
MAX_VUS="100"
DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="1200"
SEPARATE_NAMESPACES="false"
CREATE_AUTOMOUNT_RESOURCES="false"
LOGS_DIR="logs"
TEST_DURATION_IN_MINUTES="25"

# ----------- Main Execution Flow -----------
main() {
  parse_arguments "$@"
  create_namespace
  create_rbac
  start_background_watchers

  if [[ "$MODE" == "operator" ]]; then
    install_k6_operator
    create_k6_configmap
    delete_existing_testruns
    create_k6_test_run
    wait_for_test_completion
    fetch_test_logs
  elif [[ "$MODE" == "binary" ]]; then
    generate_token_and_api_url
    run_k6_binary_test
  else
    echo "❌ Invalid mode: $MODE"
    exit 1
  fi
  stop_background_watchers
  delete_namespace
}

# ----------- Helper Functions -----------
print_help() {
  cat <<EOF
Usage: $0 [options]

Options:
  --mode <operator|binary>                    Mode to run the script (default: operator)
  --max-vus <int>                             Number of virtual users for k6 (default: 100)
  --separate-namespaces <true|false>          Use separate namespaces for workspaces (default: false)
  --devworkspace-ready-timeout-seconds <int>  Timeout in seconds for workspace to become ready (default: 1200)
  --devworkspace-link <string>                DevWorkspace link (default: empty, opinionated DevWorkspace is created)
  --create-automount-resources <true|false>   Whether to create automount resources (default: false)
  --dwo-namespace <string>                    DevWorkspace Operator namespace (default: loadtest-devworkspaces)
  --logs-dir <string>                         Directory name where DevWorkspace and event logs would be dumped
  --test-duration-minutes <int>               Duration in minutes for which to run load tests (default: 25 minutes)
  -h, --help                                  Show this help message
EOF
}

parse_arguments() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --mode)
        MODE="$2"; shift 2;;
      --max-vus)
        MAX_VUS="$2"; shift 2;;
      --separate-namespaces)
        SEPARATE_NAMESPACES="$2"; shift 2;;
      --devworkspace-ready-timeout-seconds)
        DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="$2"; shift 2;;
      --devworkspace-link)
        DEVWORKSPACE_LINK="$2"; shift 2;;
      --create-automount-resources)
        CREATE_AUTOMOUNT_RESOURCES="$2"; shift 2;;
      --dwo-namespace)
        NAMESPACE="$2"; shift 2;;
      --logs-dir)
        LOGS_DIR="$2"; shift 2;;
      --test-duration-minutes)
        TEST_DURATION_IN_MINUTES="$2"; shift 2;;
      -h|--help)
        print_help; exit 0;;
      *)
        echo "❌ Unknown option: $1"
        print_help; exit 1;;
    esac
  done
}

create_namespace() {
  echo "🔧 Creating Namespace: $NAMESPACE"
  cat <<EOF | oc apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
EOF
}

delete_namespace() {
  echo "🗑️ Deleting Namespace: $NAMESPACE"
  oc delete namespace "${NAMESPACE}" --ignore-not-found
}

create_rbac() {
  echo "🔧 Creating ServiceAccount and RBAC..."
  kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${CLUSTERROLE_NAME}
rules:
  - apiGroups: ["workspace.devfile.io"]
    resources: ["devworkspaces"]
    verbs: ["create", "get", "list", "watch", "delete", "deletecollection"]
  - apiGroups: [""]
    resources: ["configmaps", "secrets", "namespaces"]
    verbs: ["create", "get", "list", "watch", "delete"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${ROLEBINDING_NAME}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${CLUSTERROLE_NAME}
subjects:
  - kind: ServiceAccount
    name: ${SA_NAME}
    namespace: ${NAMESPACE}
EOF
}

generate_token_and_api_url() {
  echo "🔐 Generating token..."
  KUBE_TOKEN=$(kubectl create token "${SA_NAME}" -n "${NAMESPACE}")

  echo "🌐 Getting Kubernetes API server URL..."
  KUBE_API=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
}

start_background_watchers() {
  echo "📁 Creating logs dir ..."
  mkdir -p ${LOGS_DIR}

  echo "🔍 Starting background watchers..."
  kubectl get events --field-selector involvedObject.kind=Pod --watch --all-namespaces \
    >> "${LOGS_DIR}/$(date +%Y-%m-%d)_events.log" 2>&1 &
  PID_EVENTS_WATCH=$!

  kubectl get dw --watch --all-namespaces \
    >> "${LOGS_DIR}/$(date +%Y-%m-%d)_dw_watch.log" 2>&1 &
  PID_DW_WATCH=$!
}

stop_background_watchers() {
  echo "🛑 Stopping background watchers..."
  kill "$PID_EVENTS_WATCH" "$PID_DW_WATCH" 2>/dev/null || true
}

install_k6_operator() {
  echo "📦 Installing k6 operator..."
  curl -L "https://raw.githubusercontent.com/grafana/k6-operator/refs/tags/${K6_OPERATOR_VERSION}/bundle.yaml" | kubectl apply -f -
  echo "⏳ Waiting until k6 operator deployment is ready..."
  kubectl rollout status deployment/k6-operator-controller-manager -n k6-operator-system --timeout=300s
}

create_k6_configmap() {
  echo "🧩 Creating ConfigMap from script file: $K6_SCRIPT"
  kubectl create configmap "$CONFIGMAP_NAME" \
    --from-file=script.js="$K6_SCRIPT" \
    --namespace "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
}

delete_existing_testruns() {
  echo "🧹 Deleting any existing K6 TestRun resources in namespace: $NAMESPACE"
  kubectl delete testrun --all -n "$NAMESPACE" || true
}

create_k6_test_run() {
  echo "🚀 Creating K6 TestRun custom resource..."
  cat <<EOF | kubectl apply -f -
apiVersion: k6.io/v1alpha1
kind: TestRun
metadata:
  name: $K6_CR_NAME
  namespace: $NAMESPACE
spec:
  parallelism: 1
  script:
    configMap:
      name: $CONFIGMAP_NAME
      file: script.js
  runner:
    serviceAccountName: $SA_NAME
    env:
    - name: IN_CLUSTER
      value: 'true'
    - name: CREATE_AUTOMOUNT_RESOURCES
      value: '${CREATE_AUTOMOUNT_RESOURCES}'
    - name: DWO_NAMESPACE
      value: '${DWO_NAMESPACE}'
    - name: SEPARATE_NAMESPACES
      value: '${SEPARATE_NAMESPACES}'
    - name: DEVWORKSPACE_LINK
      value: '${DEVWORKSPACE_LINK}'
    - name: NAMESPACE
      value: '${NAMESPACE}'
    - name: MAX_VUS
      value: '${MAX_VUS}'
    - name: TEST_DURATION_IN_MINUTES
      value: '${TEST_DURATION_IN_MINUTES}'
    - name: DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS
      value: '${DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS}'
EOF
}

wait_for_test_completion() {
  echo "⏳ Waiting for k6 TestRun to finish (timeout:${TEST_DURATION_IN_MINUTES}m)"

  TIMEOUT=$(((TEST_DURATION_IN_MINUTES+10) * 60 ))
  INTERVAL=5    # seconds

  end=$((SECONDS + TIMEOUT))

  while true; do
    stage=$(kubectl get testrun "$K6_CR_NAME" -n "$NAMESPACE" -o jsonpath='{.status.stage}' 2>/dev/null)

    if [[ "$stage" == "finished" ]]; then
        echo "TestRun $K6_CR_NAME is finished."
        break
    fi

    if (( SECONDS >= end )); then
        echo "Timeout waiting for TestRun $CR_NAME to finish."
        exit 1
    fi

    sleep "$INTERVAL"
  done
}

fetch_test_logs() {
  K6_TEST_POD=$(kubectl get pod -l k6_cr=$K6_CR_NAME,runner=true -n "${NAMESPACE}" -o jsonpath='{.items[0].metadata.name}')
  echo "📜 Fetching logs from completed K6 test pod: $K6_TEST_POD"
  kubectl logs "$K6_TEST_POD" -n "$NAMESPACE"
}

run_k6_binary_test() {
  echo "🚀 Running k6 load test..."
  IN_CLUSTER='false' \
  KUBE_TOKEN="${KUBE_TOKEN}" \
  KUBE_API="${KUBE_API}" \
  DWO_NAMESPACE="${DWO_NAMESPACE}" \
  CREATE_AUTOMOUNT_RESOURCES="${CREATE_AUTOMOUNT_RESOURCES}" \
  SEPARATE_NAMESPACES="${SEPARATE_NAMESPACES}" \
  DEVWORKSPACE_LINK="${DEVWORKSPACE_LINK}" \
  MAX_VUS="${MAX_VUS}" \
  TEST_DURATION_IN_MINUTES="${TEST_DURATION_IN_MINUTES}" \
  DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="${DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS}" \
  k6 run "${K6_SCRIPT}"
}

main "$@"
