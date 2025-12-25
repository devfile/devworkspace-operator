#!/usr/bin/env bash
set -euo pipefail

log_info()    { echo -e "ℹ️  $*" >&2; }
log_success() { echo -e "✅ $*" >&2; }
log_error()   { echo -e "❌ $*" >&2; }


run_che_ca_bundle_e2e() {
  local che_ns="$1"
  local dw_ns="$2"
  local dw_name="$3"
  local cert_count="${4:-500}"
  local bundle_file="${5:-custom-ca-certificates.pem}"

  check_namespaces "${che_ns}" "${dw_ns}"
  generate_dummy_certs "${cert_count}" "${bundle_file}"
  create_che_ca_configmap "${che_ns}" "${bundle_file}"
  patch_checluster_disable_pki_mount "${che_ns}"
  restart_che "${che_ns}"
  create_devworkspace "${dw_ns}" "${dw_name}"

  local pod
  pod=$(wait_for_workspace_pod "${dw_ns}" "${dw_name}")

  verify_ca_bundle_in_workspace "${pod}" "${dw_ns}" "${cert_count}"
  cleanup_resources "${dw_ns}" "${dw_name}"
}

check_namespaces() {
  local che_ns="$1"
  local dw_ns="$2"

  log_info "Checking namespaces..."
  kubectl get ns "${che_ns}" >/dev/null
  kubectl get ns "${dw_ns}" >/dev/null
}

generate_dummy_certs() {
  local cert_count="$1"
  local bundle_file="$2"

  log_info "Generating ${cert_count} dummy CA certificates..."
  rm -f "${bundle_file}"

  for i in $(seq 1 "${cert_count}"); do
    openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
      -subj "/CN=dummy-ca-${i}" \
      -keyout "dummy-ca-${i}.key" \
      -out "dummy-ca-${i}.pem" \
      >/dev/null 2>&1

    cat "dummy-ca-${i}.pem" >> "${bundle_file}"
  done

  log_success "Created CA bundle: $(du -h "${bundle_file}" | cut -f1)"
}

create_che_ca_configmap() {
  local che_ns="$1"
  local bundle_file="$2"

  log_info "Creating Che CA bundle ConfigMap..."

  kubectl create configmap custom-ca-certificates \
    --from-file=custom-ca-certificates.pem="${bundle_file}" \
    -n "${che_ns}" \
    --dry-run=client -o yaml \
  | kubectl apply --server-side -f -

  kubectl label configmap custom-ca-certificates \
    app.kubernetes.io/component=ca-bundle \
    app.kubernetes.io/part-of=che.eclipse.org \
    -n "${che_ns}" \
    --overwrite
}

patch_checluster_disable_pki_mount() {
  local che_ns="$1"

  log_info "Configuring CheCluster..."
  local checluster
  checluster=$(kubectl get checluster -n "${che_ns}" -o jsonpath='{.items[0].metadata.name}')

  kubectl patch checluster "${checluster}" \
    -n "${che_ns}" \
    --type=merge \
    -p '{
      "spec": {
        "devEnvironments": {
          "trustedCerts": {
            "disableWorkspaceCaBundleMount": true
          }
        }
      }
    }'
}

restart_che() {
  local che_ns="$1"

  log_info "Restarting Che..."
  kubectl rollout status deploy/che -n "${che_ns}" --timeout=5m
  kubectl wait pod -n "${che_ns}" -l app=che --for=condition=Ready --timeout=5m

  log_success "Che restarted"
}

create_devworkspace() {
  local dw_ns="$1"
  local dw_name="$2"

  log_info "Creating DevWorkspace '${dw_name}'..."
  cat <<EOF | kubectl apply -n "${dw_ns}" -f -
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: ${dw_name}
  annotations:
    che.eclipse.org/che-editor: che-incubator/che-code/latest
    che.eclipse.org/devfile: |
      schemaVersion: 2.2.0
      metadata:
        generateName: ${dw_name}
    che.eclipse.org/devfile-source: |
      url:
        location: https://github.com/che-samples/web-nodejs-sample.git
      factory:
        params: che-editor=che-incubator/che-code/latest
spec:
  started: true
  template:
    projects:
      - name: web-nodejs-sample
        git:
          remotes:
            origin: "https://github.com/che-samples/web-nodejs-sample.git"
    components:
      - name: dev
        container:
          image: quay.io/devfile/universal-developer-image:latest
          memoryLimit: 512Mi
          memoryRequest: 256Mi
          cpuRequest: 1000m
    commands:
      - id: say-hello
        exec:
          component: dev
          commandLine: echo "Hello from \$(pwd)"
          workingDir: \${PROJECT_SOURCE}/app
  contributions:
    - name: che-code
      uri: https://eclipse-che.github.io/che-plugin-registry/main/v3/plugins/che-incubator/che-code/latest/devfile.yaml
      components:
        - name: che-code-runtime-description
          container:
            env:
              - name: CODE_HOST
                value: 0.0.0.0
EOF


  kubectl wait devworkspace/"${dw_name}" \
    -n "${dw_ns}" \
    --for=condition=Ready \
    --timeout=5m
}

wait_for_workspace_pod() {
  local dw_ns="$1"
  local dw_name="$2"
  local pod_name

  log_info "Waiting for workspace pod..."

  kubectl wait pod \
    -n "${dw_ns}" \
    -l controller.devfile.io/devworkspace_name="${dw_name}" \
    --for=condition=Ready \
    --timeout=5m \
    >/dev/stderr

  pod_name=$(kubectl get pod \
    -n "${dw_ns}" \
    -l controller.devfile.io/devworkspace_name="${dw_name}" \
    -o jsonpath='{.items[0].metadata.name}')

  echo "${pod_name}"
}

verify_ca_bundle_in_workspace() {
  local pod_name="$1"
  local dw_ns="$2"
  local expected_count="$3"
  local cert_path="/public-certs/tls-ca-bundle.pem"

  log_info "Verifying CA bundle in workspace..."

  kubectl exec "${pod_name}" -n "${dw_ns}" -- test -f "${cert_path}"

  local mounted_count
  mounted_count=$(kubectl exec "${pod_name}" -n "${dw_ns}" -- \
    sh -c "grep -c 'BEGIN CERTIFICATE' ${cert_path}")

  log_info "Generated certificates : ${expected_count}"
  log_info "Mounted certificates   : ${mounted_count}"

  if [[ "${mounted_count}" -le "${expected_count}" ]]; then
    log_error "Mounted certificate count validation failed"
    return 1
  fi

  log_success "CA bundle verification passed"
}

cleanup_resources() {
  local dw_ns="$1"
  local dw_name="$2"

  log_info "Cleaning up..."
  kubectl delete dw "${dw_name}" -n "${dw_ns}" --ignore-not-found
  rm -f *.pem *.key
}
