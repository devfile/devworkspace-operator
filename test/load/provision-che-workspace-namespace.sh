provision_che_workspace_namespace() {
  local LOAD_TEST_NAMESPACE="$1"
  local CHE_NAMESPACE="$2"
  local CHE_CLUSTER_NAME="$3"

  if [[ -z "${LOAD_TEST_NAMESPACE}" ]]; then
    echo "ERROR: LOAD_TEST_NAMESPACE argument is required"
    echo "Usage: provision_che_workspace_namespace <namespace>"
    return 1
  fi

  if ! command -v oc >/dev/null 2>&1; then
    echo "ERROR: oc CLI not found"
    return 1
  fi

  local USERNAME
  USERNAME="$(oc whoami)"

  echo "Provisioning Che workspace namespace"
  echo "  User      : ${USERNAME}"
  echo "  Namespace : ${LOAD_TEST_NAMESPACE}"

  oc patch checluster "${CHE_CLUSTER_NAME}" \
    -n "${CHE_NAMESPACE}" \
    --type=merge \
    -p '{
      "spec": {
        "devEnvironments": {
          "defaultNamespace": {
            "autoProvision": false
          }
        }
      }
    }' >/dev/null

  cat <<EOF | oc apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${LOAD_TEST_NAMESPACE}
  labels:
    app.kubernetes.io/part-of: che.eclipse.org
    app.kubernetes.io/component: workspaces-namespace
  annotations:
    che.eclipse.org/username: ${USERNAME}
EOF

  oc get namespace "${LOAD_TEST_NAMESPACE}" >/dev/null

  echo "✔ Namespace '${LOAD_TEST_NAMESPACE}' provisioned for user '${USERNAME}'"
}
