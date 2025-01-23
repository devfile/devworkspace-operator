# Installation on OpenShift Without OLM

## Prerequisites

Before you begin, ensure you have the following tools installed:

*   **oc:** The OpenShift command-line tool.
*   Access to an OpenShift cluster.

## Steps

### 1. Create Namespace for Operator

Create a dedicated namespace for the DevWorkspace Controller:

```sh
oc create namespace devworkspace-controller
```

### 2. Install the DevWorkspace Operator

Install the DevWorkspace Operator using the OpenShift-specific bundle:

```sh
oc apply -f https://raw.githubusercontent.com/devfile/devworkspace-operator/refs/heads/main/deploy/deployment/openshift/combined.yaml
```

Wait until the DevWorkspace Operator pods are ready:

```sh
oc wait --namespace devworkspace-controller \
    --timeout 90s \
    --for=condition=ready pod \
    --selector=app.kubernetes.io/part-of=devworkspace-operator
```

### 3. Create DevWorkspaces Namespace

Create a namespace for the DevWorkspace sample:

```sh
oc create namespace devworkspace-samples
```


### 4. Create a Sample DevWorkspace

Create a sample DevWorkspace in the `devworkspace-samples` namespace:

```sh
oc apply -f - <<EOF
kind: DevWorkspace
apiVersion: workspace.devfile.io/v1alpha2
metadata:
  name: git-clone-sample-devworkspace
  namespace: devworkspace-samples
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

```sh
oc patch devworkspace git-clone-sample-devworkspace -n devworkspace-samples --type merge -p '{"spec": {"started": true}}'
```
You can also check the DevWorkspace status by running:
```sh
oc get devworkspace -n devworkspace-samples
```

When the DevWorkspace is running according to the status, open the editor by accesssing the URL from the `INFO` column in a web browser. For example:

```
NAME                            DEVWORKSPACE ID             PHASE     INFO
git-clone-sample-devworkspace   workspace0196ce197f0b4e90   Running   <URL>
```
