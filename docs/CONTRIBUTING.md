# Contributing to DevWorkspace Operator

Hello there! Thank you for choosing to contribute to DevWorkspace Operator. Navigate through the following table of contents to learn more about contributing to the project.

- [Contributing to DevWorkspace Operator](#contributing-to-devworkspace-operator)
- [How to Contribute](#how-to-contribute)
  - [Set up your Development Environment](#set-up-your-development-environment)
      - [Running the controller locally.](#running-the-controller-locally)
  - [Testing Changes](#testing-changes)
      - [Test run the controller](#test-run-the-controller)
      - [Developing Webhooks](#developing-webhooks)
  - [Signing-off on Commits](#signing-off-on-commits)

# How to Contribute
To contribute to the DevWorkspace Operator project, developers should follow the [fork](https://docs.github.com/en/get-started/quickstart/fork-a-repo) and [pull request workflow](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request-from-a-fork). 
## Set up your Development Environment


**1.** Fork [devworkspace-operator](https://github.com/devfile/devworkspace-operator) repository.

**2.** Clone your forked copy of the project.

```
git clone https://github.com/<your-github-username>/devworkspace-operator.git
```

#### Running the controller locally.

In the steps listed below, we set up the development environment using a [minikube cluster](https://minikube.sigs.k8s.io/docs/start/). 

**1.** Start the minikube cluster.

```
minikube start
```

**2.** Enable the ingress add-on for your minikube cluster.

```
minikube addons enable ingress
```

**3.** Set the namespace environment variable for the development environment to avoid changes inside the default namespace.

```
export NAMESPACE="devworkspace-controller"
```

**4.** Install the kubernetes certificate management controller to generate and manage TLS certificates for your cluster. 

```
make install_cert_manager.
```
Please note that the above step is not specific to minikube. The cert-manager is required for all deployments on Kubernetes.


**5.** Install the dependencies for running the DevWorkspace Operator in your cluster.

```
make install
```

**6.** Scale down the replicas controller-manager pods to 0.

```
kubectl patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}" -n $NAMESPACE
```

**7.** Run the devworkspace-operator.

```
make run
```

This will run the devworkspace-controller on your local system.

**8.** Make your changes in the new branch and test the changes.


## Testing Changes

#### Test run the controller

**1.** Take a look samples DevWorkspaces in the `./samples` directory. If you are uncertain on which one to try, use `theia-latest.yaml`.

**2.** Apply a sample by executing `kubectl apply -f ./samples/<SAMPLE-FILENAME> -n $NAMESPACE`. For instance, `kubectl apply -f ./samples/theia-latest.yaml -n  $NAMESPACE`.

**3.** As soon as devworkspace is started you're able to get IDE url by executing `kubectl get devworkspace -n <namespace>`.

**4.** To check for the DevWorkspace status, execute `kubectl get dw -n $NAMESPACE -w`.

**5.** As soon as the DevWorkspace is started, an IDE url will appear from the output of `kubectl get dw -n $NAMESPACE -w`. This assumes that the DevWorkspace sample you chose includes an IDE.

#### Developing Webhooks

**1.** Make a change to the webhook.


**2.** Ensure the `DWO_IMG` environment variable points to your container image repository, eg. export `DWO_IMG=quay.io/<username>/dwo-webhook:next`.


**3.** Run `make docker restart` (assuming DWO is already deployed to the cluster, otherwise run `make docker install`).
Wait for the webhook deployment to update with your image that contains your latest changes.
## Signing-off on Commits

To contribute to this project, you must agree to the **code of conduct** for each commit you make. 

See the [code of conduct](https://github.com/devfile/api/blob/main/CODE_OF_CONDUCT.md) file for the full text of what you must agree to.
To signify that you agree to the DCO for contributions, you simply add a line to each of your
git commit messages:

```
Signed-off-by: John Doe <john.doe@example.com>
```

**Note:** you don't have to manually include this line on your commits, git does that for you as shown below:

```
$ git commit -s -m “my commit message w/signoff”
```

In most cases, git automatically adds the signoff to your commit with the use of
`-s` or `--signoff` flag to `git commit`. You must use your real name and a reachable email
address (sorry, no pseudonyms or anonymous contributions).

To ensure all your commits are signed, you may choose to add this alias to your global `.gitconfig`:

_~/.gitconfig_

```
[alias]
  amend = commit -s --amend
  cm = commit -s -m
  commit = commit -s
```
