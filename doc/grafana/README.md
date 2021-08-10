# Grafana dashboards for the DevWorkspace Operator

This directory contains a sample Grafana dashboard that processes some of the prometheus metrics exposed by the DevWorkspace Operator. Once Prometheus and Grafana are configured with the DevWorkspace Operator's metrics server as a source, the dashboard can be imported from [json](grafana-dashbaord.json).

## Testing DevWorkspace Operator metrics locally

To quickly test metrics locally (without installing Prometheus Grafana in the cluster), it's possible to deploy prometheus and grafana as containers locally.

1. Create a clusterrolebinding for a service account to allow the metrics-reader role (this example uses the controller's service account)
    ```bash
    NAMESPACE=devworkspace-controller # Namespace where controller is installed
    kubectl create clusterrolebinding dw-metrics \
      --clusterrole=devworkspace-controller-metrics-reader \
      --serviceaccount=${NAMESPACE}:devworkspace-controller-serviceaccount
    ```
2. Get the serviceaccount's token to enable local access to metrics
    ```bash
    TOKEN=$(kubectl get secrets -o=json -n ${NAMESPACE} | jq -r '[.items[] |
      select (
        .type == "kubernetes.io/service-account-token" and
        .metadata.annotations."kubernetes.io/service-account.name" == "devworkspace-controller-serviceaccount")][0].data.token' \
      | base64 --decode)
    ```
3. In another terminal, use `kubectl port-forward` to expose the controller's service locally
    ```bash
    kubectl port-forward service/devworkspace-controller-metrics 8443:8443 &;
    kubectl port-forward service/devworkspace-webhookserver 9443:9443
    ```
    We can check that the token works and metrics are accessible:
    ```bash
    curl -k -H "Authorization: Bearer ${TOKEN}" https://localhost:9443/metrics
    curl -k -H "Authorization: Bearer ${TOKEN}" https://localhost:8443/metrics
    ```
4. Create a prometheus config (optionally in a tmp dir) to configure prometheus to use the SA token
    ```yaml
    cat <<EOF > prometheus.yaml
    global:
      scrape_interval:     3s
      evaluation_interval: 3s

    scrape_configs:
      - job_name: 'DevWorkspace'
        scheme: https
        authorization:
          type: Bearer
          credentials: ${TOKEN}
        tls_config:
          insecure_skip_verify: true
        static_configs:
        - targets: ['localhost:8443']

      - job_name: 'DevWorkspace webhooks'
        scheme: https
        authorization:
          type: Bearer
          credentials: ${TOKEN}
        tls_config:
          insecure_skip_verify: true
        static_configs:
        - targets: ['localhost:9443']
    EOF
    ```
5. Start prometheus in a container locally
    ```bash
    docker run -d --name prometheus \
      --network=host \
      -v $(pwd)/prometheus.yaml:/etc/prometheus/prometheus.yaml:z \
      prom/prometheus \
      --config.file=/etc/prometheus/prometheus.yaml \
      --web.listen-address=:9999 \
      --log.level=debug
    ```
    note: `--network=host` is required to enable the docker container to access `localhost` correctly (otherwise, `localhost` is within the container)
6. Start grafana in a container locally
    ```bash
    docker run -d --name grafana -p 3000:3000 grafana/grafana
    ```
7. Navigate to `localhost:3000`, login as `admin/admin`, add the datasource for prometheus (`http://localhost:9999`, `Access: Browser`), and import the dashboard.
