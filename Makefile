# Copyright (c) 2019-2024 Red Hat, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

SHELL := bash
.SHELLFLAGS = -ec
.ONESHELL:
.DEFAULT_GOAL := help

ifndef VERBOSE
  MAKEFLAGS += --silent
endif

export NAMESPACE ?= devworkspace-controller
export DWO_IMG ?= quay.io/devfile/devworkspace-controller:next
export DWO_BUNDLE_IMG ?= quay.io/devfile/devworkspace-operator-bundle:next
export DWO_INDEX_IMG ?= quay.io/devfile/devworkspace-operator-index:next
export PROJECT_CLONE_IMG ?= quay.io/devfile/project-clone:next
export PULL_POLICY ?= Always
export DEFAULT_ROUTING ?= basic
export KUBECONFIG ?= ${HOME}/.kube/config
export DEVWORKSPACE_API_VERSION ?= a6ec0a38307b63a29fad2eea945cc69bee97a683

# Enable using Podman instead of Docker
export DOCKER ?= docker

#internal params
DEVWORKSPACE_CTRL_SA=devworkspace-controller-serviceaccount
INTERNAL_TMP_DIR=/tmp/devworkspace-controller
BUMPED_KUBECONFIG=$(INTERNAL_TMP_DIR)/kubeconfig
CONTROLLER_ENV_FILE=$(INTERNAL_TMP_DIR)/environment

include build/make/version.mk
include build/make/olm.mk

ifneq (,$(shell which kubectl 2>/dev/null)$(shell which oc 2>/dev/null))
  include build/make/deploy.mk
endif

OPERATOR_SDK_VERSION = v1.8.0
OPM_VERSION = v1.19.5

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1,trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
  GOBIN=$(shell go env GOPATH)/bin
else
  GOBIN=$(shell go env GOBIN)
endif

all: help

_print_vars:
	@echo "Current env vars:"
	@echo "    NAMESPACE=$(NAMESPACE)"
	@echo "    DWO_IMG=$(DWO_IMG)"
	@echo "    DWO_BUNDLE_IMG=$(DWO_BUNDLE_IMG)"
	@echo "    DWO_INDEX_IMG=$(DWO_INDEX_IMG)"
	@echo "    PROJECT_CLONE_IMG=$(PROJECT_CLONE_IMG)"
	@echo "    PULL_POLICY=$(PULL_POLICY)"
	@echo "    ROUTING_SUFFIX=$(ROUTING_SUFFIX)"
	@echo "    DEFAULT_ROUTING=$(DEFAULT_ROUTING)"
	@echo "    DEVWORKSPACE_API_VERSION=$(DEVWORKSPACE_API_VERSION)"
	@echo "Build environment:"
	@echo "    Build Time:       $(BUILD_TIME)"
	@echo "    Go Package Path:  $(GO_PACKAGE_PATH)"
	@echo "    Architecture:     $(ARCH)"
	@echo "    Git Commit SHA-1: $(GIT_COMMIT_ID)"

##### Rules for dealing with devfile/api
### update_devworkspace_api: Updates the version of devworkspace crds in go.mod
update_devworkspace_api:
	go mod edit --require github.com/devfile/api/v2@$(DEVWORKSPACE_API_VERSION)
	go mod download
	go mod tidy

_init_devworkspace_crds:
	./update_devworkspace_crds.sh --init --api-version $(DEVWORKSPACE_API_VERSION)

### update_devworkspace_crds: Pulls the latest devworkspace CRDs to ./devworkspace-crds. Note: pulls master branch
update_devworkspace_crds:
	./update_devworkspace_crds.sh --api-version $(DEVWORKSPACE_API_VERSION)
###### End rules for dealing with devfile/api

### test: Runs tests
test: generate fmt vet manifests envtest
  ifneq ($(shell command -v ginkgo 2> /dev/null),)
	  go test $(shell go list ./... | grep -v test/e2e | grep -v controllers/workspace) -coverprofile cover.out
	  KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	    ginkgo run --timeout 5m --randomize-all -coverprofile controller.cover.out controllers/workspace controllers/controller/devworkspacerouting
  else
	  KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	    go test $(shell go list ./... | grep -v test/e2e) -coverprofile cover.out
  endif

### test_e2e: Runs e2e test on the cluster set in context. DevWorkspace Operator must be already deployed
test_e2e:
	mkdir -p /tmp/artifacts
	CGO_ENABLED=0 go test -v -c -o bin/devworkspace-controller-e2e ./test/e2e/cmd/workspaces_test.go
	./bin/devworkspace-controller-e2e -ginkgo.fail-fast --ginkgo.junit-report=/tmp/artifacts/junit-workspaces-operator.xml

### test_e2e_debug: Runs e2e test in debug mode, so it's possible to connect to execution via remote debugger
test_e2e_debug:
	mkdir -p /tmp/artifacts
	dlv test --listen=:2345 --headless=true --api-version=2 ./test/e2e/cmd/workspaces_test.go -- --ginkgo.fail-fast --ginkgo.junit-report=/tmp/artifacts/junit-workspaces-operator.xml

### manager: Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

### generate_all: regenerate all resources for operator (CRDs, manifests, bundle, etc.)
generate_all: generate manifests generate_default_deployment generate_olm_bundle_yaml

### generate_deployment: Generates the files used for deployment from kustomize templates, using environment variables
generate_deployment:
	build/scripts/generate_deployment.sh

### generate_default_deployment: Generates the files used for deployment from kustomize templates with default values
generate_default_deployment:
	build/scripts/generate_deployment.sh --use-defaults

### manifests: Generates the manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=role webhook paths="./..." \
	    output:crd:artifacts:config=deploy/templates/crd/bases \
	    output:rbac:artifacts:config=deploy/templates/components/rbac
	patch/patch_crds.sh

### fmt: Runs go fmt against code
fmt:
  ifneq ($(shell command -v goimports 2> /dev/null),)
	  find . -name '*.go' -exec goimports -w {} \;
  else
	  @echo "WARN: goimports is not installed -- formatting using go fmt instead."
	  @echo "      Please install goimports to ensure file imports are consistent."
	  go fmt -x ./...
  endif

### fmt_license: Ensures the license header is set on all files
fmt_license:
  ifneq ($(shell command -v addlicense 2> /dev/null),)
	  @echo 'addlicense -v -f license_header.txt **/*.go'
	  addlicense -v -f license_header.txt $$(find . -name '*.go')
  else
	  $(error addlicense must be installed for this rule: go get -u github.com/google/addlicense)
  endif

### check_fmt: Checks the formatting on files in repo
check_fmt:
  ifeq ($(shell command -v goimports 2> /dev/null),)
	  $(error "goimports must be installed for this rule" && exit 1)
  endif
  ifeq ($(shell command -v addlicense 2> /dev/null),)
	  $(error "error addlicense must be installed for this rule: go get -u github.com/google/addlicense")
  endif
	@{
	  if [[ $$(find . -name '*.go' -exec goimports -l {} \;) != "" ]]; then \
	    echo "Files not formatted; run 'make fmt'"; exit 1 ;\
	  fi ;\
	  if ! addlicense -check -f license_header.txt $$(find . -name '*.go'); then \
	    echo "Licenses are not formatted; run 'make fmt_license'"; exit 1 ;\
	  fi \
	}

### vet: Runs go vet against code
vet:
	go vet ./...

### generate: Generates code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	@$(MAKE) fmt

### docker: Builds and pushes controller image
docker: _print_vars docker-build docker-push

### docker-build: Builds the controller image
docker-build:
	$(DOCKER) build . -t ${DWO_IMG} -f build/Dockerfile

### docker-push: Pushes the controller image
docker-push:
  ifneq ($(INITIATOR),CI)
    ifeq ($(DWO_IMG),quay.io/devfile/devworkspace-controller:next)
	    @echo -n "Are you sure you want to push $(DWO_IMG)? [y/N] " && read ans && [ $${ans:-N} = y ]
    endif
  endif
	$(DOCKER) push ${DWO_IMG}

### compile-devworkspace-controller: Compiles the devworkspace-controller binary
.PHONY: compile-devworkspace-controller
compile-devworkspace-controller:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) GO111MODULE=on go build \
	  -a -o _output/bin/devworkspace-controller \
	  -gcflags all=-trimpath=/ \
	  -asmflags all=-trimpath=/ \
	  -ldflags "-X $(GO_PACKAGE_PATH)/version.Commit=$(GIT_COMMIT_ID) \
	  -X $(GO_PACKAGE_PATH)/version.BuildTime=$(BUILD_TIME)" \
	main.go

### compile-webhook-server: Compiles the webhook-server
.PHONY: compile-webhook-server
compile-webhook-server:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) GO111MODULE=on go build \
	  -o _output/bin/webhook-server \
	  -gcflags all=-trimpath=/ \
	  -asmflags all=-trimpath=/ \
	  -ldflags "-X $(GO_PACKAGE_PATH)/version.Commit=$(GIT_COMMIT_ID) \
	  -X $(GO_PACKAGE_PATH)/version.BuildTime=$(BUILD_TIME)" \
	webhook/main.go

.PHONY: help
### help: Prints this message
help: Makefile
	@echo 'Available rules:'
	@sed -n 's/^### /    /p' $(MAKEFILE_LIST) | awk 'BEGIN { FS=":" } { printf "%-30s -%s\n", $$1, $$2 }'
	@echo ''
	@echo 'Supported environment variables:'
	@echo '    DWO_IMG                    - Image used for controller'
	@echo '    PROJECT_CLONE_IMG          - Image used for project-clone init container'
	@echo '    NAMESPACE                  - Namespace to use for deploying controller'
	@echo '    KUBECONFIG                 - Kubeconfig which should be used for accessing to the cluster. Currently is: $(KUBECONFIG)'
	@echo '    ROUTING_SUFFIX             - Cluster routing suffix (e.g. $$(minikube ip).nip.io, apps-crc.testing)'
	@echo '    PULL_POLICY                - Image pull policy for controller'
	@echo '    DEVWORKSPACE_API_VERSION   - Branch or tag of the github.com/devfile/api to depend on. Defaults to master'

# Automatic setup of required binaries: controller-gen, envtest
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_GEN_VERSION = v0.6.1
ENVTEST ?= $(LOCALBIN)/setup-envtest
ENVTEST_K8S_VERSION = 1.24.2

### controller-gen: Finds or downloads controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
