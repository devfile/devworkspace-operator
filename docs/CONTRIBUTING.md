# Contributing to Devworkspace-Operator

Hello there! Thank you for choosing to the contributing to devfile/devworkspace-operator. Navigate through the following to understand more about contributing here.

- [Contributing to Devworkspace-Operator](#contributing-to-devworkspace-operator)
- [Before You Get Started](#before-you-get-started)
  - [Code of Conduct](#code-of-conduct)
  - [For Newcomers](#for-newcomers)
- [How to Contribute](#how-to-contribute)
  - [Set up your Local Development Environment](#set-up-your-local-development-environment)
  - [Testing Changes](#testing-changes)
  - [Signing-off on Commits](#signing-off-on-commits)
    (#testing-your-changes)
  - [Signing-off on Commits](#signing-off-on-commits)

# Before You Get Started

## Code of Conduct

## For Newcomers

# How to Contribute

<!--
## Prerequisites

Make sure you have the following prerequisites installed on your operating system before you start contributing:

- [Nodejs and npm](https://nodejs.org/en/)

  To verify run:

  ```
  node -v
  ```

  ```
  npm -v
  ```

- [Gatsby.js](https://www.gatsbyjs.com/)

  To verify run:

  ```
  gatsby --version
  ```

**Note:** If you're on a _Windows environment_ then it is highly recommended that you install [Windows Subsystem for Linux (WSL)](https://docs.microsoft.com/en-us/windows/wsl/install) both for performance and ease of use. Refer to the [documentation](https://docs.microsoft.com/en-us/windows/dev-environment/javascript/gatsby-on-wsl) for the installation of _Gatsby.js on WSL_. -->

## Set up your Local Development Environment

Follow the following instructions to start contributing.

**1.** Fork [this](https://github.com/devfile/devworkspace-operator) repository.

**2.** Clone your forked copy of the project.

```
git clone https://github.com/<your-github-username>/devworkspace-operator.git
```

**3.** Navigate to the project directory.

```
cd devworkspace-operator
```

**4.** Add a reference(remote) to the original repository.

```
git remote add upstream https://github.com/devfile/devworkspace-operator.git
```

**5.** Check the remotes for this repository.

```
git remote -v
```

**6.** Always take a pull from the upstream repository to your master branch to keep it at par with the main project (updated repository).

```
git pull upstream master
```

**7.** Create a new branch.

```
git checkout -b <your_branch_name>
```

**8.** Start the minikube cluster.

```
minikube start
```

**9.** Enable the ingress add-on for your minikube cluster.

```
minikube addons enable ingress
```

**10.** Set the namespace environment variable for the development environment to avoid changes inside the default namespace.

```
export NAMESPACE="devworkspace-controller"
```

**11.** Install the kubernetes certificate management controller to generate and manage TLS certificates for your cluster.

```
make install cert-manager.
```

**12.** Install the dependencies for running the devworkspace-operator in your cluster.

```
make install
```

**13.** Scale down the replicas of pods to 0.

```
kubectl patch deployment/devworkspace-controller-manager --patch "{\"spec\":{\"replicas\":0}}" -n $NAMESPACE
```

**14.** Run the devworkspace-operator.

```
make run
```

This will run the devworkspace-controller in your local cluster (minikube/openshift).

**15.** Make your changes in the new branch and trach the changes.

```
git add .
```

**16.** Commit your changes. To contribute to this project, you must agree to the [Developer Certificate of Origin (DCO)](#signing-off-on-commits) for each commit you make.

```
git commit --signoff -m "<commit subject>"
```

or you could go with the shorter format for the same, as shown below.

```
git commit -s -m "<commit subject>"
```

**17.** While you are working on your branch, other developers may update the `master` branch with their branch. This action means your branch is now out of date with the `master` branch and missing content. So to fetch the new changes, follow along:

```
git checkout master
git fetch origin master
git merge upstream/master
git push origin
```

Now you need to merge the `master` branch into your branch. This can be done in the following way:

```
git checkout <your_branch_name>
git merge master
```

**18.** Push the committed changes in your feature branch to your remote repo.

```
git push -u origin <your_branch_name>
```

## Testing Changes

## Signing-off on Commits

To contribute to this project, you must agree to the **Developer Certificate of
Origin (DCO)** for each commit you make. The DCO is a simple statement that you,
as a contributor, have the legal right to make the contribution.

See the [DCO](https://developercertificate.org) file for the full text of what you must agree to
and how it works [here](https://github.com/probot/dco#how-it-works).
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
