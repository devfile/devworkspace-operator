# Copyright (c) 2019-2025 Red Hat, Inc.
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
export PROJECT_BACKUP_IMG ?= quay.io/devfile/project-clone:next
export PULL_POLICY ?= Always
export DEFAULT_ROUTING ?= basic
export KUBECONFIG ?= ${HOME}/.kube/config
export DEVWORKSPACE_API_VERSION ?= a6ec0a38307b63a29fad2eea945cc69bee97a683

# Container tool detection: auto-detect running Docker or Podman, allow override
# Skip detection if we're inside a container build (when container tools aren't needed)
ifeq (,$(DOCKER))
  # Check for running services first (prefer Docker for performance)
  ifneq (,$(shell docker info >/dev/null 2>&1 && echo "running"))
    export DOCKER := docker
    export CONTAINER_TOOL := docker
  else ifneq (,$(shell podman info >/dev/null 2>&1 && echo "running"))
    export DOCKER := podman
    export CONTAINER_TOOL := podman
  else
    # Fallback: check if binaries are installed but not running
    ifneq (,$(shell which docker 2>/dev/null))
      ifneq (,$(shell which podman 2>/dev/null))
        # Both installed but neither running
        ifeq ($(filter docker% _docker% build_bundle_and_index,$(MAKECMDGOALS)),)
          export DOCKER := not-available
          export CONTAINER_TOOL := not-available
        else
          $(error Both Docker and Podman are installed but neither is running. Please start Docker Desktop or Podman machine)
        endif
      else
        # Only Docker installed but not running
        ifeq ($(filter docker% _docker% build_bundle_and_index,$(MAKECMDGOALS)),)
          export DOCKER := not-available
          export CONTAINER_TOOL := not-available
        else
          $(error Docker is installed but not running. Please start Docker Desktop)
        endif
      endif
    else ifneq (,$(shell which podman 2>/dev/null))
      # Only Podman installed but not running
      ifeq ($(filter docker% _docker% build_bundle_and_index,$(MAKECMDGOALS)),)
        export DOCKER := not-available
        export CONTAINER_TOOL := not-available
      else
        $(error Podman is installed but not running. Please start Podman machine)
      endif
    else
      # Neither installed
      ifeq ($(filter docker% _docker% build_bundle_and_index,$(MAKECMDGOALS)),)
        export DOCKER := not-available
        export CONTAINER_TOOL := not-available
      else
        $(error Neither Docker nor Podman found. Please install Docker Desktop or Podman)
      endif
    endif
  endif
else
  export CONTAINER_TOOL := $(DOCKER)
endif

# Check if buildx is available for Docker (only if docker is available)
ifeq ($(CONTAINER_TOOL),docker)
  BUILDX_AVAILABLE := $(shell docker buildx version >/dev/null 2>&1 && echo true || echo false)
else
  BUILDX_AVAILABLE := false
endif

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

CRD_OPTIONS ?= "crd:crdVersions=v1"

# Default to linux for container builds, but allow override
GOOS ?= linux
# GOARCH is set dynamically in build/make/version.mk

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
	@echo "    PROJECT_BACKUP_IMG=$(PROJECT_BACKUP_IMG)"
	@echo "    PULL_POLICY=$(PULL_POLICY)"
	@echo "    ROUTING_SUFFIX=$(ROUTING_SUFFIX)"
	@echo "    DEFAULT_ROUTING=$(DEFAULT_ROUTING)"
	@echo "    DEVWORKSPACE_API_VERSION=$(DEVWORKSPACE_API_VERSION)"
	@echo "Container tool:"
	@echo "    CONTAINER_TOOL=$(CONTAINER_TOOL)"
	@echo "    BUILDX_AVAILABLE=$(BUILDX_AVAILABLE)"
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

### docker-build: Builds the multi-arch image (supports both amd64 and arm64)
docker-build:
	@echo "Building multi-arch image ${DWO_IMG} for linux/amd64,linux/arm64 using $(CONTAINER_TOOL)"
ifeq ($(CONTAINER_TOOL),docker)
  ifeq ($(BUILDX_AVAILABLE),false)
	$(error Docker buildx is required for multi-arch builds. Please update Docker or enable buildx)
  endif
	@echo "Using Docker buildx to build multi-arch images"
	$(MAKE) _docker-build-amd64 _docker-build-arm64
	@echo "✅ Built multi-arch images locally:"
	@echo "  ${DWO_IMG}-amd64"
	@echo "  ${DWO_IMG}-arm64"
	@echo "Note: Manifest list will be created during push to registry"
else
	@echo "Using Podman to build multi-arch image"
	$(MAKE) _docker-build-amd64 _docker-build-arm64
	@echo "Creating manifest list for ${DWO_IMG} using Podman"
	@echo "Cleaning up any existing images/manifests with the same name"
	@$(DOCKER) manifest rm ${DWO_IMG} 2>/dev/null || echo "    (manifest not found, continuing)"
	@$(DOCKER) rmi ${DWO_IMG} 2>/dev/null || echo "    (image not found, continuing)"
	$(DOCKER) manifest create ${DWO_IMG} ${DWO_IMG}-amd64 ${DWO_IMG}-arm64
endif

### _docker-build-amd64: Builds the amd64 image
_docker-build-amd64:
ifeq ($(CONTAINER_TOOL),docker)
  ifeq ($(BUILDX_AVAILABLE),false)
	$(error Docker buildx is required for platform-specific builds. Please update Docker or enable buildx)
  endif
	$(DOCKER) buildx build . --platform linux/amd64 --load -t ${DWO_IMG}-amd64 -f build/Dockerfile
else
	$(DOCKER) build . --platform linux/amd64 -t ${DWO_IMG}-amd64 -f build/Dockerfile
endif

### _docker-build-arm64: Builds the arm64 image
_docker-build-arm64:
ifeq ($(CONTAINER_TOOL),docker)
  ifeq ($(BUILDX_AVAILABLE),false)
	$(error Docker buildx is required for platform-specific builds. Please update Docker or enable buildx)
  endif
	$(DOCKER) buildx build . --platform linux/arm64 --load -t ${DWO_IMG}-arm64 -f build/Dockerfile
else
	$(DOCKER) build . --platform linux/arm64 -t ${DWO_IMG}-arm64 -f build/Dockerfile
endif

### docker-build-amd64: Builds only the amd64 image (for single-arch builds)
docker-build-amd64: _docker-build-amd64

### docker-build-arm64: Builds only the arm64 image (for single-arch builds)
docker-build-arm64: _docker-build-arm64

### docker-push-amd64: Pushes only the amd64 image (for single-arch workflows)
docker-push-amd64: _docker-check-push
	$(DOCKER) push ${DWO_IMG}-amd64

### docker-push-arm64: Pushes only the arm64 image (for single-arch workflows)
docker-push-arm64: _docker-check-push
	$(DOCKER) push ${DWO_IMG}-arm64


### docker-push: Pushes the multi-arch image to the registry
docker-push: _docker-check-push
	@echo "Pushing multi-arch image ${DWO_IMG} using $(CONTAINER_TOOL)"
ifeq ($(CONTAINER_TOOL),docker)
  ifeq ($(BUILDX_AVAILABLE),false)
	$(error Docker buildx is required for multi-arch pushes. Please update Docker or enable buildx)
  endif
	@echo "Using Docker buildx to push multi-arch image"
	$(DOCKER) push ${DWO_IMG}-amd64
	$(DOCKER) push ${DWO_IMG}-arm64
	@echo "Creating and pushing manifest list using Docker buildx"
	$(DOCKER) buildx imagetools create -t ${DWO_IMG} ${DWO_IMG}-amd64 ${DWO_IMG}-arm64
else
	@echo "Using Podman to push multi-arch image"
	$(DOCKER) push ${DWO_IMG}-amd64
	$(DOCKER) push ${DWO_IMG}-arm64
	@echo "Cleaning up any existing manifests before recreating"
	@$(DOCKER) manifest rm ${DWO_IMG} 2>/dev/null || echo "    (manifest not found, continuing)"
	$(DOCKER) manifest create ${DWO_IMG} ${DWO_IMG}-amd64 ${DWO_IMG}-arm64
	$(DOCKER) manifest push ${DWO_IMG}
endif

### _docker-check-push: Asks for confirmation before pushing the image, unless running in CI
_docker-check-push:
  ifneq ($(INITIATOR),CI)
    ifeq ($(DWO_IMG),quay.io/devfile/devworkspace-controller:next)
	    @echo -n "Are you sure you want to push $(DWO_IMG)? [y/N] " && read ans && [ $${ans:-N} = y ]
    endif
  endif

### compile-devworkspace-controller: Compiles the devworkspace-controller binary
.PHONY: compile-devworkspace-controller
compile-devworkspace-controller:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build \
	  -a -o _output/bin/devworkspace-controller \
	  -gcflags all=-trimpath=/ \
	  -asmflags all=-trimpath=/ \
	  -ldflags "-X $(GO_PACKAGE_PATH)/version.Commit=$(GIT_COMMIT_ID) \
	  -X $(GO_PACKAGE_PATH)/version.BuildTime=$(BUILD_TIME)" \
	main.go

### compile-webhook-server: Compiles the webhook-server
.PHONY: compile-webhook-server
compile-webhook-server:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build \
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
	@echo '    PROJECT_BACKUP_IMG         - Image used for project-backup workspace backup container'
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
CONTROLLER_GEN_VERSION = v0.18.0
ENVTEST_VERSION = v0.0.0-20240320141353-395cfc7486e6
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
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
