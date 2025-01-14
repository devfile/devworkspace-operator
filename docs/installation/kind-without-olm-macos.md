# Installation on Kind Without OLM (MacOs)

## Prerequisites

Before you begin, ensure you have the following tools installed:

*   **kubectl:** The Kubernetes command-line tool.
*   **kind:** A tool for running Kubernetes locally using Docker.
*   **OrbStack** (as a container runtime)

## Steps

### 1. Create Kind Cluster

Create a Kind cluster:

```sh
kind create cluster
```

### 2. Install MetalLB

Install MetalLB using the provided manifest:

```sh
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml
```

Wait until the MetalLB pods are ready:

```sh
kubectl wait --namespace metallb-system \
  --for=condition=ready pod \
  --selector=app=metallb \
  --timeout=90s
```

### 3. Define MetalLB IP Address Pool

Inspect the Docker network to find the subnet range:

```sh
docker network inspect kind
```

In the output, look for the `Subnet` entry (e.g., `"Subnet": "192.168.97.0/24"`). Then, define an IP address pool using a range within that subnet.

**Note:** The Subnet will be different on your host, please use your own Subnet!
For example if you have `"Subnet": "192.168.97.0/24"`, then it could be `192.168.97.100-192.168.97.110`.

Apply the IP address pool and L2 advertisement:

```sh
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: ip-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.97.100-192.168.97.110
EOF
```

```sh
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - ip-pool
EOF
```

### 4. Install NGINX Ingress Controller

Install the NGINX ingress controller:

```sh
kubectl apply -f https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml
```

Wait until the NGINX ingress controller pods are ready:

```sh
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

### 5. Create Namespace

Create a dedicated namespace for the DevWorkspace Controller:

```sh
kubectl create namespace devworkspace-controller
```

### 6. Install cert-manager

Install cert-manager using the provided manifest:

```sh
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.4/cert-manager.yaml
```

Wait until cert-manager pods are ready:

```sh
kubectl wait --namespace cert-manager \
      --timeout 90s \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/instance=cert-manager
```

### 7. Install the DevWorkspace Operator

Install the DevWorkspace Operator from the given URL:

```sh
kubectl apply -f https://github.com/devfile/devworkspace-operator/raw/refs/tags/v0.31.2/deploy/deployment/kubernetes/combined.yaml
```

Wait until the DevWorkspace Operator pods are ready:

```sh
kubectl wait --namespace devworkspace-controller \
      --timeout 90s \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/part-of=devworkspace-operator
```

### 8. Create the DevWorkspace Operator Config

#### 8.1 Get Load Balancer IP
Get the Load Balancer IP from the `ingress-nginx` service:

```sh
kubectl get services \
   --namespace ingress-nginx \
   ingress-nginx-controller \
   --output jsonpath='{.status.loadBalancer.ingress[0].ip}'
```
Let's denote this value as `<HOST_IP>`.

#### 8.2 Create the DevWorkspaceOperatorConfig

Create the `DevWorkspaceOperatorConfig` resource, replacing `<HOST_IP>` with the IP you obtained in the previous step:

```sh
kubectl apply -f - <<EOF
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: devworkspace-controller
config:
  routing:
    clusterHostSuffix: "<HOST_IP>.nip.io"
EOF
```

### 9. Create a Sample DevWorkspace

Create a sample DevWorkspace:

```sh
kubectl apply -f - <<EOF
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: git-clone-sample-devworkspace
spec:
  started: true
  template:
    projects:
      - name: web-nodejs-sample
        git:
          remotes:
            origin: "https://github.com/che-samples/web-nodejs-sample.git"
      - name: devworkspace-operator
        git:
          checkoutFrom:
            remote: origin
            revision: 0.21.x
          remotes:
            origin: "https://github.com/devfile/devworkspace-operator.git"
            amisevsk: "https://github.com/amisevsk/devworkspace-operator.git"
    commands:
      - id: say-hello
        exec:
          component: che-code-runtime-description
          commandLine: echo "Hello from $(pwd)"
          workingDir: ${PROJECT_SOURCE}/app
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
```

**Note:** The DevWorkspace start may fail due to timeout if some container images are large and take longer than 5 minutes to pull. If the DevWorkspace fails, you can restart it by setting `spec.started` to `true`. Use the following command to re-trigger the DevWorkspace start:

```bash
kubectl patch devworkspace git-clone-sample-devworkspace -n default --type merge -p '{"spec": {"started": true}}'
```
You can also check the DevWorkspace status by running:
```sh
kubectl get devworkspace -n default
```
