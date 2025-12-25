#!/bin/bash

source test/load/provision-che-workspace-namespace.sh
source test/load/che-cert-bundle-utils.sh


MODE="binary"  # or 'operator'
LOAD_TEST_NAMESPACE="loadtest-devworkspaces"
DWO_NAMESPACE="openshift-operators"
SA_NAME="k6-devworkspace-tester"
CLUSTERROLE_NAME="k6-devworkspace-role"
ROLEBINDING_NAME="k6-devworkspace-binding"
CONFIGMAP_NAME="k6-test-script"
K6_CR_NAME="k6-test-run"
K6_SCRIPT="test/load/devworkspace_load_test.js"
K6_OPERATOR_VERSION="v0.0.22"
DEVWORKSPACE_LINK="https://gist.githubusercontent.com/rohanKanojia/ecf625afaf3fe817ac7d1db78bd967fc/raw/8c30c0370444040105ca45cd4ac0f7062a644bb7/dw-minimal.json"
MAX_VUS="100"
DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="1200"
SEPARATE_NAMESPACES="false"
DELETE_DEVWORKSPACE_AFTER_READY="true"
MAX_DEVWORKSPACES="-1"
CREATE_AUTOMOUNT_RESOURCES="false"
RUN_WITH_ECLIPSE_CHE="false"
LOGS_DIR="logs"
TEST_DURATION_IN_MINUTES="25"
MIN_KUBECTL_VERSION="1.24.0"
MIN_CURL_VERSION="7.0.0"
MIN_K6_VERSION="1.1.0"
CHE_NAMESPACE="eclipse-che"
CHE_CLUSTER_NAME="eclipse-che"
TEST_CERTIFICATES_COUNT="500"

# ----------- Main Execution Flow -----------
main() {
  parse_arguments "$@"
  check_prerequisites
  if [[ "$RUN_WITH_ECLIPSE_CHE" == "false" ]]; then
    create_namespace
  else
    provision_che_workspace_namespace "$LOAD_TEST_NAMESPACE" "$CHE_NAMESPACE" "$CHE_CLUSTER_NAME"
    run_che_ca_bundle_e2e "$CHE_NAMESPACE" "$LOAD_TEST_NAMESPACE" "test-devworkspace" "$TEST_CERTIFICATES_COUNT"
  fi
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
  --max-devworkspaces <int>                   Maximum number of DevWorkspaces to create (by default, it's not specified)
  --separate-namespaces <true|false>          Use separate namespaces for workspaces (default: false)
  --delete-devworkspace-after-ready           Delete DevWorkspace once it becomes Ready (default: true)
  --devworkspace-ready-timeout-seconds <int>  Timeout in seconds for workspace to become ready (default: 1200)
  --devworkspace-link <string>                DevWorkspace link (default: empty, opinionated DevWorkspace is created)
  --create-automount-resources <true|false>   Whether to create automount resources (default: false)
  --dwo-namespace <string>                    DevWorkspace Operator namespace (default: loadtest-devworkspaces)
  --logs-dir <string>                         Directory name where DevWorkspace and event logs would be dumped
  --test-duration-minutes <int>               Duration in minutes for which to run load tests (default: 25 minutes)
  --run-with-eclipse-che <true|false>         Whether these tests are supposed to be run with Eclipse Che (If yes additional certificates are mounted)
  --che-cluster-name <string>                 Applicable if running on Eclipse Che, defaults to 'eclipse-che'
  --che-namespace <string>                    Applicable if running on Eclipse Che, defaults to 'eclipse-che'
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
      --max-devworkspaces)
        MAX_DEVWORKSPACES="$2"; shift 2;;
      --delete-devworkspace-after-ready)
        DELETE_DEVWORKSPACE_AFTER_READY="$2"; shift 2;;
      --devworkspace-ready-timeout-seconds)
        DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="$2"; shift 2;;
      --devworkspace-link)
        DEVWORKSPACE_LINK="$2"; shift 2;;
      --create-automount-resources)
        CREATE_AUTOMOUNT_RESOURCES="$2"; shift 2;;
      --dwo-namespace)
        LOAD_TEST_NAMESPACE="$2"; shift 2;;
      --logs-dir)
        LOGS_DIR="$2"; shift 2;;
      --test-duration-minutes)
        TEST_DURATION_IN_MINUTES="$2"; shift 2;;
      --run-with-eclipse-che)
        RUN_WITH_ECLIPSE_CHE="$2"; shift 2;;
      --che-cluster-name)
        CHE_CLUSTER_NAME="$2"; shift 2;;
      --che-namespace)
        CHE_NAMESPACE="$2"; shift 2;;
      -h|--help)
        print_help; exit 0;;
      *)
        echo "❌ Unknown option: $1"
        print_help; exit 1;;
    esac
  done
}

create_namespace() {
  echo "🔧 Creating Namespace: $LOAD_TEST_NAMESPACE"
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${LOAD_TEST_NAMESPACE}
EOF
}

delete_namespace() {
  echo "🗑️ Deleting Namespace: $LOAD_TEST_NAMESPACE"
  kubectl delete namespace "${LOAD_TEST_NAMESPACE}" --ignore-not-found --wait=false
}

create_rbac() {
  echo "🔧 Creating ServiceAccount and RBAC..."
  kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  namespace: ${LOAD_TEST_NAMESPACE}
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
    namespace: ${LOAD_TEST_NAMESPACE}
EOF
}

generate_token_and_api_url() {
  echo "🔐 Generating token..."
  KUBE_TOKEN=$(kubectl create token "${SA_NAME}" -n "${LOAD_TEST_NAMESPACE}")

  echo "🌐 Getting Kubernetes API server URL..."
  KUBE_API=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
}

start_background_watchers() {
  echo "📁 Creating logs dir ..."
  mkdir -p ${LOGS_DIR}
  TIMESTAMP=$(date +%Y-%m-%d_%H-%M-%S)

  echo "🔍 Starting background watchers..."
  kubectl get events --field-selector involvedObject.kind=Pod --watch --all-namespaces \
    >> "${LOGS_DIR}/${TIMESTAMP}_events.log" 2>&1 &
  PID_EVENTS_WATCH=$!

  kubectl get dw --watch --all-namespaces \
    >> "${LOGS_DIR}/${TIMESTAMP}_dw_watch.log" 2>&1 &
  PID_DW_WATCH=$!

  log_failed_devworkspaces &
  PID_FAILED_DW_POLL=$!
}

log_failed_devworkspaces() {
  echo "📄 Starting periodic failed DevWorkspaces report (every 10s)..."

  POLL_INTERVAL=10  # in seconds
  ITERATIONS=$((((TEST_DURATION_IN_MINUTES-1) * 60) / POLL_INTERVAL))

  for ((i = 0; i < ITERATIONS; i++)); do
    OUTPUT=$(kubectl get devworkspaces --all-namespaces -o json | jq -r '
      .items[]
      | select(.status.phase == "Failed")
      | [
          .metadata.namespace,
          .metadata.name,
          .status.phase,
          (.status.message // "No message")
        ]
      | @csv')

    if [ -n "$OUTPUT" ]; then
      echo "$OUTPUT" > "${LOGS_DIR}/dw_failure_report.csv"
    fi

    sleep "$POLL_INTERVAL"
  done
}

stop_background_watchers() {
  echo "🛑 Stopping background watchers..."
  kill "$PID_EVENTS_WATCH" "$PID_DW_WATCH" "$PID_FAILED_DW_POLL" 2>/dev/null || true
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
    --namespace "$LOAD_TEST_NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
}

delete_existing_testruns() {
  echo "🧹 Deleting any existing K6 TestRun resources in namespace: $LOAD_TEST_NAMESPACE"
  kubectl delete testrun --all -n "$LOAD_TEST_NAMESPACE" || true
}

create_k6_test_run() {
  echo "🚀 Creating K6 TestRun custom resource..."
  cat <<EOF | kubectl apply -f -
apiVersion: k6.io/v1alpha1
kind: TestRun
metadata:
  name: $K6_CR_NAME
  namespace: $LOAD_TEST_NAMESPACE
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
    - name: LOAD_TEST_NAMESPACE
      value: '${LOAD_TEST_NAMESPACE}'
    - name: MAX_VUS
      value: '${MAX_VUS}'
    - name: TEST_DURATION_IN_MINUTES
      value: '${TEST_DURATION_IN_MINUTES}'
    - name: DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS
      value: '${DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS}'
    - name: DELETE_DEVWORKSPACE_AFTER_READY
      value: '${DELETE_DEVWORKSPACE_AFTER_READY}'
    - name: MAX_DEVWORKSPACES
      value: '${MAX_DEVWORKSPACES}'
EOF
}

wait_for_test_completion() {
  echo "⏳ Waiting for k6 TestRun to finish (timeout:${TEST_DURATION_IN_MINUTES}m)"

  TIMEOUT=$(((TEST_DURATION_IN_MINUTES+10) * 60 ))
  INTERVAL=5    # seconds

  end=$((SECONDS + TIMEOUT))

  while true; do
    stage=$(kubectl get testrun "$K6_CR_NAME" -n "$LOAD_TEST_NAMESPACE" -o jsonpath='{.status.stage}' 2>/dev/null)

    if [[ "$stage" == "finished" ]]; then
        echo "TestRun $K6_CR_NAME is finished."
        break
    fi

    if (( SECONDS >= end )); then
        echo "Timeout waiting for TestRun $K6_CR_NAME to finish."
        exit 1
    fi

    sleep "$INTERVAL"
  done
}

fetch_test_logs() {
  K6_TEST_POD=$(kubectl get pod -l k6_cr=$K6_CR_NAME,runner=true -n "${LOAD_TEST_NAMESPACE}" -o jsonpath='{.items[0].metadata.name}')
  echo "📜 Fetching logs from completed K6 test pod: $K6_TEST_POD"
  kubectl logs "$K6_TEST_POD" -n "$LOAD_TEST_NAMESPACE"
}

check_prerequisites() {
  echo "🔍 Checking prerequisites..."

  check_command "kubectl" "$MIN_KUBECTL_VERSION"
  check_command "curl" "$MIN_CURL_VERSION"
  
  if [[ "$MODE" == "binary" ]]; then
    check_command "k6" "$MIN_K6_VERSION"
  fi
}

check_command() {
  local cmd="$1"
  local min_version="$2"
  local version

  if ! command -v "$cmd" &>/dev/null; then
    echo "❌ Required command '$cmd' not found in PATH."
    exit 1
  fi

  case "$cmd" in
    kubectl)
      version=$(kubectl version --client -o json | jq -r '.clientVersion.gitVersion' | sed 's/^v//')
      ;;
    curl)
      version=$($cmd --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1)
      ;;
    k6)
      version=$($cmd version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
      ;;
    *)
      version="0.0.0"
      ;;
  esac

  if ! version_gte "$version" "$min_version"; then
    echo "❌ $cmd version $version is less than required $min_version"
    exit 1
  else
    echo "✅ $cmd version $version (>= $min_version)"
  fi
}

version_gte() {
  [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

run_k6_binary_test() {
  echo "🚀 Running k6 load test..."
  IN_CLUSTER='false' \
  KUBE_TOKEN="${KUBE_TOKEN}" \
  KUBE_API="${KUBE_API}" \
  DWO_NAMESPACE="${DWO_NAMESPACE}" \
  CREATE_AUTOMOUNT_RESOURCES="${CREATE_AUTOMOUNT_RESOURCES}" \
  SEPARATE_NAMESPACES="${SEPARATE_NAMESPACES}" \
  LOAD_TEST_NAMESPACE="${LOAD_TEST_NAMESPACE}" \
  DEVWORKSPACE_LINK="${DEVWORKSPACE_LINK}" \
  MAX_VUS="${MAX_VUS}" \
  TEST_DURATION_IN_MINUTES="${TEST_DURATION_IN_MINUTES}" \
  DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS="${DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS}" \
  DELETE_DEVWORKSPACE_AFTER_READY="${DELETE_DEVWORKSPACE_AFTER_READY}" \
  MAX_DEVWORKSPACES="${MAX_DEVWORKSPACES}" \
  k6 run "${K6_SCRIPT}"
  exit_code=$?
  if [ $exit_code -ne 0 ]; then
    echo "⚠️ k6 load test failed with exit code $exit_code. Proceeding to cleanup."
  fi
  return 0
}

trap stop_background_watchers EXIT
main "$@"
