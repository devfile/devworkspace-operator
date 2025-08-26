# Contributing

Contributions are welcome!

## Code of Conduct

Before contributing to this repository for the first time, please review our project's [Code of Conduct](https://github.com/devfile/api/blob/main/CODE_OF_CONDUCT.md).

## Getting Started

### Issues

- Open or search for [issues](https://github.com/devfile/devworkspace-operator/issues).
- If a related issue doesn't exist, you can open a new issue using a relevant [issue form](https://github.com/devfile/devworkspace-operator/issues/new/choose).

### Pull Requests

All commits must be signed off with the footer:

```git
Signed-off-by: Firstname Lastname <email@email.com>
```

Once you set your `user.name` and `user.email` in your git config, you can sign your commit automatically with
`git commit -s`. When you think the code is ready for review, create a pull request and link the issue associated with
it.

Owners of the repository will watch out for and review new PRs.

If comments have been given in a review, they have to be addressed before merging.

After addressing review comments, don't forget to add a comment in the PR afterward, so everyone gets notified by Github and knows to re-review.

## CI

### GitHub actions

- [Next Dockerimage](https://github.com/devfile/devworkspace-operator/blob/main/.github/workflows/dockerimage-next.yml) action builds main branch and pushes it to [quay.io/devfile/devworkspace-controller:next](https://quay.io/repository/devfile/devworkspace-controller?tag=latest&tab=tags)
- [Code Coverage Report](./.github/workflows/code-coverage.yml) action creates a code coverage report using [codecov.io](https://about.codecov.io/).

## Development

Detailed instructions regarding the DevWorkspace Operator development are provided in this section.

### Prerequisites

To build, test and debug the DevWorkspace Operator the following development tools are required:

- go 1.16 or later
- git
- sed
- jq
- yq (python-yq from <https://github.com/kislyuk/yq#installation>, other distributions may not work)
- skopeo (if building the OLM catalogsource)
- podman or docker

Note: kustomize `v4.0.5` is required for most tasks. It is downloaded automatically to the `.kustomize` folder in this
repo when required. This downloaded version is used regardless of whether or not kustomize is already installed on the
system.

#### macOS Specific Issues

On macOS, the default `make` utility might be outdated, leading to issues with some `Makefile` targets. To resolve this, it's recommended to install a newer version of `make` using Homebrew and ensure it's prioritized in your system's `$PATH`.

1. Install Homebrew `make`:

    ```bash
    brew install make
    ```

2. Add the Homebrew `make` executable to your `$PATH` by adding the following line to your shell configuration file (e.g., `~/.zshrc`, `~/.bash_profile`):

    ```bash
    export PATH="/opt/homebrew/opt/make/libexec/gnubin:$PATH"
    ```

After adding, reload your shell configuration (e.g., `source ~/.zshrc` or `source ~/.bash_profile`) or open a new terminal session.

### Makefile

The repository contains a `Makefile`; building and deploying can be configured via the environment variables:

|variable|purpose|default value|
|---|---|---|
| `DWO_IMG` | Image used for controller | `quay.io/devfile/devworkspace-controller:next` |
| `DEFAULT_DWO_IMG` | Image used for controller when generating default deployment templates. Can be used to override the controller image in the OLM bundle | `quay.io/devfile/devworkspace-controller:next` |
| `NAMESPACE` | Namespace to use for deploying controller | `devworkspace-controller` |
| `ROUTING_SUFFIX` | Cluster routing suffix (e.g. `$(minikube ip).nip.io`, `apps-crc.testing`). Required for Kubernetes | `192.168.99.100.nip.io` |
| `PULL_POLICY` | Image pull policy for controller | `Always` |
| `DEVWORKSPACE_API_VERSION` | Branch or tag of the github.com/devfile/api to depend on | `v1alpha1` |

Some of the rules supported by the `Makefile`:

|rule|purpose|
|---|---|
| docker | build and push docker image |
| install | install controller to cluster |
| restart | restart cluster controller deployment |
| install_cert_manager | installs the cert-manager to the cluster (only required for Kubernetes) |
| uninstall | delete controller namespace `devworkspace-controller` and remove CRDs from cluster |
| help | print all rules and variables |

To see all rules supported by the makefile, run `make help`

### DevWorkspace Operator first time development setup

1. Fork [devfile/devworkspce-operator](https://github.com/devfile/devworkspace-operator) and clone your fork locally
2. Export the `DWO_IMG` environment variable. For example:

    ```bash
    export DWO_IMG=quay.io/mloriedo/devworkspace-controller:dev
    ```

   :warning: _You need write privileges on this container registry repository. The DevWorkspace controller image will be
pushed there during build._
3. If your changes include some update to the Devfile or DevWorkspace schema set some environment variables and run
`go mod` to point to your fork instead of devfile/api:

    ```bash
    export DEVFILE_API_REPO=github.com/l0rd/api   # <== your devfile/api fork
    export DEVFILE_API_BRANCH=my-branch-name      # <== the branch of your fork
    export DEVWORKSPACE_API_VERSION=$(git ls-remote https://${DEVFILE_API_REPO} | grep refs/heads/${DEVFILE_API_BRANCH} | cut -f 1)
    go mod edit -replace github.com/devfile/api/v2=${DEVFILE_API_REPO}/v2@${DEVWORKSPACE_API_VERSION} && \
    go mod download && \
    go mod tidy
    ```

4. Build the controller go code, build the container image and publish it to the container registry:

    ```bash
    make docker
    ```

5. Install cert-manager (can be skipped on OpenShift):

    ```bash
    make install_cert_manager && \
    kubectl wait --for=condition=Available -n cert-manager deployment/cert-manager
    ```

6. Finally deploys the CRDs and the controller to the current cluster:

    ```bash
    make install  # <== this command copies the CRDs definition
                  #     creates the namespace for the controller in the cluster
                  #     downloads and runs kustomize to build the manifests
                  #     deploys all the manifests (CRDs and controller)
    ```

### Test run controller

1. Take a look samples devworkspace configuration in `./samples` folder.
2. Apply any of them by executing `kubectl apply -f ./samples/code-latest.yaml -n <namespace>`
3. As soon as devworkspace is started you're able to get IDE url by executing `kubectl get devworkspace -n <namespace>`

### Run controller locally

```bash
export NAMESPACE="devworkspace-controller"
make install
# Wait for webhook server to start
kubectl rollout status deployment devworkspace-controller-manager -n $NAMESPACE --timeout 90s
kubectl rollout status deployment devworkspace-webhook-server -n $NAMESPACE --timeout 90s
# Scale on-cluster deployment to zero to avoid conflict with locally-running instance
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}" -n $NAMESPACE
make run
```

> Note: The operator requires internet access from containers to work. By default, `crc setup` may not provision this, so it's necessary to configure DNS for Docker:
>
> ```text
> # /etc/docker/daemon.json
> {
>   "dns": ["192.168.0.1"]
> }
> ```

By default, the controller will expose workspace servers without any authentication; this is not advisable for public
clusters, as any user could access the created workspace via URL.

### Run controller locally and debug

Debugging the controller depends on [delve](https://github.com/go-delve/delve) being installed
(`go install github.com/go-delve/delve/cmd/dlv@latest`). Note that `$GOPATH/bin` or `$GOBIN` must be added to `$PATH` in
order for `make debug` to run correctly.

```bash
make install
# Wait for webhook server to start
kubectl rollout status deployment devworkspace-controller-manager -n $NAMESPACE --timeout 90s
kubectl rollout status deployment devworkspace-webhook-server -n $NAMESPACE --timeout 90s
oc patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}"
# Scale on-cluster deployment to zero to avoid conflict with locally-running instance
make debug
```

### Run webhook server locally and debug

Debugging the webhook server depends on `telepresence` being installed (`https://www.telepresence.io/docs/latest/install/`). Teleprescence works by redirecting traffic going from the webhook-server in the cluster to the local webhook-server you will be running on your computer.

```bash
make debug-webhook-server
```

when you are done debugging you have to manually uninstall the telepresence agent

```bash
make disconnect-debug-webhook-server
```

### Updating devfile API

[devfile API](https://github.com/devfile/api) is the Kube-native API for cloud development workspaces specification and the core dependency of the devworkspace-operator that should be regularly updated to the latest version. In order to do the update:

1. update `DEVWORKSPACE_API_VERSION` variable in the `Makefile` and `build/scripts/generate_deployment.sh`. The variable should correspond to the commit SHA from the [devfile API](https://github.com/devfile/api) repository
2. run the following scripts and the open pull request

```bash
make update_devworkspace_api update_devworkspace_crds # first commit
make generate_all # second commit
```

Example of the devfile API update [PR](https://github.com/devfile/devworkspace-operator/pull/797)

### Remove controller from your K8s/OS Cluster

To uninstall the controller and associated CRDs, use the Makefile uninstall rule:

```bash
make uninstall
```

This will delete all custom resource definitions created for the controller, as well as the `devworkspace-controller` namespace.

### Build a custom OLM bundle

In order to build a custom bundle, the following environment variables should be set:

| variable | purpose | default value |
|---|---|---|
| `DWO_BUNDLE_IMG` | Image used for Operator bundle image | `quay.io/devfile/devworkspace-operator-bundle:next` |
| `DWO_INDEX_IMG` | Image used for Operator index image | `quay.io/devfile/devworkspace-operator-index:next` |
| `DEFAULT_DWO_IMG` | Image used for controller when generating defaults | `quay.io/devfile/devworkspace-controller:next` |

To build the index image and register its catalogsource to the cluster, run

```bash
make generate_olm_bundle_yaml build_bundle_and_index register_catalogsource
```

Note that setting `DEFAULT_DWO_IMG` while generating sources will result in local changes to the repo which should be `git restored` before committing. This can also be done by unsetting the `DEFAULT_DWO_IMG` env var and re-running `make generate_olm_bundle_yaml`
