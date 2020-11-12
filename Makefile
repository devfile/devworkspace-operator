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

SHELL := bash
.SHELLFLAGS = -ec

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
OPERATOR_SDK_VERSION = v1.1.0
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

all: help

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
			'.spec.template.spec.containers[]?.env[] | select(.name | startswith("RELATED_IMAGE")) | "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		> $(RELATED_IMAGES_FILE)
	cat $(RELATED_IMAGES_FILE)

##### Rules for dealing with devfile/api
### update_devworkspace_api: update version of devworkspace crds in go.mod
update_devworkspace_api:
	go mod edit --require github.com/devfile/api@$(DEVWORKSPACE_API_VERSION)
	go mod download
	go mod tidy

_init_devworkspace_crds:
	./update_devworkspace_crds.sh --init --api-version $(DEVWORKSPACE_API_VERSION)

### update_devworkspace_crds: pull latest devworkspace CRDs to ./devworkspace-crds. Note: pulls master branch
update_devworkspace_crds:
	./update_devworkspace_crds.sh --api-version $(DEVWORKSPACE_API_VERSION)
###### End rules for dealing with devfile/api

### test: Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test $(shell go list ./... | grep -v test/e2e) -coverprofile cover.out

### test_e2e: runs e2e test on the cluster set in context. Includes deploying devworkspace-controller, run test workspace, uninstall devworkspace-controller
test_e2e: generate fmt vet manifests
	CGO_ENABLED=0 go test -v -c -o bin/devworkspace-controller-e2e ./test/e2e/cmd/workspaces_test.go
	./bin/devworkspace-controller-e2e

### manager: Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

### run: Run against the configured Kubernetes cluster in ~/.kube/config
run: _print_vars _generate_related_images_env
	source $(RELATED_IMAGES_FILE)
	WATCH_NAMESPACE=$(NAMESPACE) go run ./main.go

debug: _print_vars _generate_related_images_env
	source $(RELATED_IMAGES_FILE)
	WATCH_NAMESPACE=$(NAMESPACE) dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --

### install_crds: Install CRDs into a cluster
install_crds: manifests _kustomize _init_devworkspace_crds
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

### deploy: Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: _print_vars _kustomize _init_devworkspace_crds _create_namespace deploy_registry
	mv config/devel/kustomization.yaml config/devel/kustomization.yaml.bak
	mv config/devel/config.properties config/devel/config.properties.bak
	mv config/devel/manager_image_patch.yaml config/devel/manager_image_patch.yaml.bak

	envsubst < config/devel/kustomization.yaml.bak > config/devel/kustomization.yaml
	envsubst < config/devel/config.properties.bak > config/devel/config.properties
	envsubst < config/devel/manager_image_patch.yaml.bak > config/devel/manager_image_patch.yaml
	$(KUSTOMIZE) build config/devel | kubectl apply -f - || true

	mv config/devel/kustomization.yaml.bak config/devel/kustomization.yaml
	mv config/devel/config.properties.bak config/devel/config.properties
	mv config/devel/manager_image_patch.yaml.bak config/devel/manager_image_patch.yaml

### restart: Restart devworkspace-controller deployment
restart:
	kubectl rollout restart -n $(NAMESPACE) deployment/devworkspace-controller-manager

### uninstall: Remove controller resources from the cluster
uninstall: _kustomize
# It's safer to delete all workspaces before deleting the controller; otherwise we could
# leave workspaces in a hanging state if we add finalizers.
	kubectl delete devworkspaces.workspace.devfile.io --all-namespaces --all | true
	kubectl delete devworkspacetemplates.workspace.devfile.io --all-namespaces --all | true
# Have to wait for routings to be deleted in case there are finalizers
	kubectl delete workspaceroutings.controller.devfile.io --all-namespaces --all --wait | true
	kustomize build config/devel | kubectl delete --ignore-not-found -f -
	kubectl delete all -l "app.kubernetes.io/part-of=devworkspace-operator" --all-namespaces
	kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	kubectl delete validatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	kubectl delete namespace $(NAMESPACE) --ignore-not-found

### deploy_registry: Deploy plugin registry
deploy_registry: _print_vars _create_namespace
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

### docker: Build and push controller image
docker: _print_vars docker-build docker-push

### docker-build: Build the controller image
docker-build:
	docker build . -t ${IMG} -f build/Dockerfile

### docker-push: Push the controller image
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

_operator_sdk:
ifneq ($(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2),$(OPERATOR_SDK_VERSION))
	@echo 'WARN: operator-sdk $(OPERATOR_SDK_VERSION) is expected to be used for this target but $(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2) found.'
	@echo 'WARN: Please use the recommended operator-sdk if you face any issue.'
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests _operator_sdk
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
