# Installation on Kind Without OLM (Linux)

## Prerequisites

Before you begin, ensure you have the following tools installed:

* **kubectl:** The Kubernetes command-line tool.
* **kind:** A tool for running Kubernetes locally using Docker.
* **Docker** (as a container runtime)

## Steps

### 1. Create Kind Cluster with Extra Port Mappings

Create a Kind cluster with port mappings for HTTP and HTTPS traffic:

```sh
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF
```

### 2. Install NGINX Ingress Controller

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

Redeploy the `ingress-nginx-controller` service to change its type from `LoadBalancer` to `NodePort`:

```sh
kubectl delete service ingress-nginx-controller -n ingress-nginx
```

```sh
kubectl expose deployment ingress-nginx-controller --name=ingress-nginx-controller --port=80 --type=NodePort -n ingress-nginx
```

### 3. Create Namespace

Create a dedicated namespace for the DevWorkspace Controller:

```sh
kubectl create namespace devworkspace-controller
```

### 4. Install cert-manager

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

### 5. Install the DevWorkspace Operator

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

### 6. Create the DevWorkspace Operator Config

#### 6.1 Get Kind Node IP

Get the internal IP address of your Kind control-plane node:

```sh
kubectl get node -o wide
```

Look for the `INTERNAL-IP` of the `kind-control-plane` node. Let's denote this as `<HOST_IP>`. You will use this IP in the next step.

#### 6.2 Create the DevWorkspaceOperatorConfig

Create the `DevWorkspaceOperatorConfig` resource, replacing `<HOST_IP>` with the IP you obtained in the previous step:

```bash
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

### 7. Create a Sample DevWorkspace

Create a sample DevWorkspace:

```bash
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

**Note:** The DevWorkspace creation may fail due to timeout if some container images are large and take longer than 5 minutes to pull. If the DevWorkspace fails, you can restart it by setting `spec.started` to `true`. Use the following command to re-trigger the DevWorkspace start:

```bash
kubectl patch devworkspace git-clone-sample-devworkspace -n default --type merge -p '{"spec": {"started": true}}'
```

You can also check the DevWorkspace status by running:

```sh
kubectl get devworkspace -n default
```

When the DevWorkspace is running according to the status, open the editor by accesssing the URL from the `INFO` column in a web browser. For example:

```sh
NAME                            DEVWORKSPACE ID             PHASE     INFO
git-clone-sample-devworkspace   workspace0196ce197f0b4e90   Running   <URL>
```
