# Copyright (c) 2019-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#

#### TODOS ####
## - Port over e2e tests.
####

SHELL := bash
.SHELLFLAGS = -ec
# .ONESHELL:

export NAMESPACE ?= devworkspace-controller
export IMG ?= quay.io/devfile/devworkspace-controller:next
export ROUTING_SUFFIX ?= 192.168.99.100.nip.io
export PULL_POLICY ?= Always
export WEBHOOK_ENABLED ?= true
export DEFAULT_ROUTING ?= basic
REGISTRY_ENABLED ?= true
DEVWORKSPACE_API_VERSION ?= v1alpha1

#internal params
INTERNAL_TMP_DIR=/tmp/devworkspace-controller
BUMPED_KUBECONFIG=$(INTERNAL_TMP_DIR)/kubeconfig
RELATED_IMAGES_FILE=$(INTERNAL_TMP_DIR)/environment

ifeq ($(shell kubectl api-resources --api-group='route.openshift.io' | grep -o routes),routes)
PLATFORM := openshift
else
PLATFORM := kubernetes
endif

# minikube handling
ifeq ($(shell kubectl config current-context),minikube)
export ROUTING_SUFFIX := $(shell minikube ip).nip.io
endif


# Bootstrapped by Operator-SDK v1.1.0
# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

_print_vars:
	@echo "Current env vars:"
	@echo "    NAMESPACE=$(NAMESPACE)"
	@echo "    IMG=$(IMG)"
	@echo "    PULL_POLICY=$(PULL_POLICY)"
	@echo "    ROUTING_SUFFIX=$(ROUTING_SUFFIX)"
	@echo "    WEBHOOK_ENABLED=$(WEBHOOK_ENABLED)"
	@echo "    DEFAULT_ROUTING=$(DEFAULT_ROUTING)"
	@echo "    REGISTRY_ENABLED=$(REGISTRY_ENABLED)"
	@echo "    DEVWORKSPACE_API_VERSION=$(DEVWORKSPACE_API_VERSION)"

_create_namespace:
	kubectl create namespace $(NAMESPACE) || true

_generate_related_images_env:
	@mkdir -p $(INTERNAL_TMP_DIR)
	cat ./config/components/manager/manager.yaml \
		| yq -r \
			'.spec.template.spec.containers[].env[] | select(.name | startswith("RELATED_IMAGE")) | "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		> $(RELATED_IMAGES_FILE)
	cat $(RELATED_IMAGES_FILE)

##### Rules for dealing with devfile/api
### update_devworkspace_api: update version of devworkspace crds in go.mod
update_devworkspace_api:
	go mod edit --require github.com/devfile/api@$(DEVWORKSPACE_API_VERSION)
	go mod download
	go mod tidy

### update_devworkspace_crds: pull latest devworkspace CRDs to ./devworkspace-crds. Note: pulls master branch
update_devworkspace_crds:
	./update_devworkspace_crds.sh $(DEVWORKSPACE_API_VERSION)
###### End rules for dealing with devfile/api

### test: Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

### manager: Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

### run: Run against the configured Kubernetes cluster in ~/.kube/config
run: _generate_related_images_env
	source $(RELATED_IMAGES_FILE)
	go run ./main.go

debug: _generate_related_images_env
	source $(RELATED_IMAGES_FILE)
	dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --

### install: Install CRDs into a cluster
install: manifests _kustomize _create_namespace
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

#### uninstall: Uninstall CRDs from a cluster
uninstall: manifests _kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

### deploy: Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: _kustomize _create_namespace deploy_registry
	mv config/devel/config.properties config/devel/config.properties.bak
	mv config/devel/manager_image_patch.yaml config/devel/manager_image_patch.yaml.bak
	envsubst < config/devel/config.properties.bak > config/devel/config.properties
	envsubst < config/devel/manager_image_patch.yaml.bak > config/devel/manager_image_patch.yaml
	$(KUSTOMIZE) build config/devel | kubectl apply -f - || true
	mv config/devel/config.properties.bak config/devel/config.properties
	mv config/devel/manager_image_patch.yaml.bak config/devel/manager_image_patch.yaml

### restart: Restart devworkspace-controller deployment
restart:
	kubectl rollout restart -n $(NAMESPACE) deployment/devworkspace-controller-manager

### uninstall_controller: Remove controller resources from the cluster
uninstall_controller: _kustomize
	kustomize build config/devel | kubectl delete --ignore-not-found -f -

### deploy_registry: Deploy plugin registry
deploy_registry: _create_namespace
	kubectl apply -f config/registry/local -n $(NAMESPACE)
ifeq ($(PLATFORM),kubernetes)
	envsubst < config/registry/local/k8s/ingress.yaml | kubectl apply -n $(NAMESPACE) -f -
else
	kubectl apply -f config/registry/local/os -n $(NAMESPACE)
endif

### manifests: Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." \
			output:crd:artifacts:config=config/crd/bases \
			output:rbac:artifacts:config=config/components/rbac
	patch/patch_crds.sh

### fmt: Run go fmt against code
fmt:
ifneq ($(shell command -v goimports 2> /dev/null),)
	find . -name '*.go' -exec goimports -w {} \;
else
	@echo "WARN: goimports is not installed -- formatting using go fmt instead."
	@echo "      Please install goimports to ensure file imports are consistent."
	go fmt -x ./...
endif

### fmt_license: ensure license header is set on all files
fmt_license:
ifneq ($(shell command -v addlicense 2> /dev/null),)
	@echo 'addlicense -v -f license_header.txt **/*.go'
	@addlicense -v -f license_header.txt $$(find . -name '*.go')
else
	$(error addlicense must be installed for this rule: go get -u github.com/google/addlicense)
endif

### vet: Run go vet against code
vet:
	go vet ./...

### generate: Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

### docker-build: Build the docker image
docker-build:
	docker build . -t ${IMG} -f build/Dockerfile

### docker-push: Push the docker image
docker-push:
	docker push ${IMG}

### controller-gen: find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

_kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: help
### help: print this message
help: Makefile
	@echo 'Available rules:'
	@sed -n 's/^### /    /p' $< | awk 'BEGIN { FS=":" } { printf "%-30s -%s\n", $$1, $$2 }'
	@echo ''
	@echo 'Supported environment variables:'
	@echo '    IMG                        - Image used for controller'
	@echo '    NAMESPACE                  - Namespace to use for deploying controller'
	@echo '    ROUTING_SUFFIX             - Cluster routing suffix (e.g. $$(minikube ip).nip.io, apps-crc.testing)'
	@echo '    PULL_POLICY                - Image pull policy for controller'
	@echo '    WEBHOOK_ENABLED            - Whether webhooks should be enabled in the deployment'
	@echo '    ADMIN_CTX                  - Kubectx entry that should be used during work with cluster. The current will be used if omitted'
	@echo '    REGISTRY_ENABLED           - Whether the plugin registry should be deployed'
	@echo '    DEVWORKSPACE_API_VERSION   - Branch or tag of the github.com/devfile/api to depend on. Defaults to master'
