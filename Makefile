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
.ONESHELL:

ifndef VERBOSE
MAKEFLAGS += --silent
endif

export NAMESPACE ?= devworkspace-controller
export IMG ?= quay.io/devfile/devworkspace-controller:next
export ROUTING_SUFFIX ?= 192.168.99.100.nip.io
export PULL_POLICY ?= Always
export WEBHOOK_ENABLED ?= true
export DEFAULT_ROUTING ?= basic
REGISTRY_ENABLED ?= true
DEVWORKSPACE_API_VERSION ?= aeda60d4361911da85103f224644bfa792498499

#internal params
INTERNAL_TMP_DIR=/tmp/devworkspace-controller
BUMPED_KUBECONFIG=$(INTERNAL_TMP_DIR)/kubeconfig
RELATED_IMAGES_FILE=$(INTERNAL_TMP_DIR)/environment

ifeq (,$(shell which kubectl))
ifeq (,$(shell which oc))
$(error oc or kubectl is required to proceed)
else
K8S_CLI := oc
endif
else
K8S_CLI := kubectl
endif

ifeq ($(shell $(K8S_CLI) api-resources --api-group='route.openshift.io' | grep -o routes),routes)
PLATFORM := openshift
else
PLATFORM := kubernetes
endif

# minikube handling
ifeq ($(shell $(K8S_CLI) config current-context),minikube)
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
	@echo "    IMG=$(IMG)"
	@echo "    PULL_POLICY=$(PULL_POLICY)"
	@echo "    ROUTING_SUFFIX=$(ROUTING_SUFFIX)"
	@echo "    WEBHOOK_ENABLED=$(WEBHOOK_ENABLED)"
	@echo "    DEFAULT_ROUTING=$(DEFAULT_ROUTING)"
	@echo "    REGISTRY_ENABLED=$(REGISTRY_ENABLED)"
	@echo "    DEVWORKSPACE_API_VERSION=$(DEVWORKSPACE_API_VERSION)"

_create_namespace:
	$(K8S_CLI) create namespace $(NAMESPACE) || true

_gen_configuration_env:
	mkdir -p $(INTERNAL_TMP_DIR)
	echo "export RELATED_IMAGE_devworkspace_webhook_server=$(IMG)" > $(RELATED_IMAGES_FILE)
ifeq ($(PLATFORM),kubernetes)
	echo "export WEBHOOK_SECRET_NAME=devworkspace-operator-webhook-cert" >> $(RELATED_IMAGES_FILE)
endif
	cat ./config/components/manager/manager.yaml \
		| yq -r \
			'.spec.template.spec.containers[]?.env[] | select(.name | startswith("RELATED_IMAGE")) | "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		>> $(RELATED_IMAGES_FILE)
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
test_e2e:
	CGO_ENABLED=0 go test -v -c -o bin/devworkspace-controller-e2e ./test/e2e/cmd/workspaces_test.go
	./bin/devworkspace-controller-e2e

### manager: Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# it's easier to bump whole kubeconfig instead of grabbing cluster URL from the current context
_bump_kubeconfig:
	mkdir -p $(INTERNAL_TMP_DIR)
ifndef KUBECONFIG
	$(eval CONFIG_FILE = ${HOME}/.kube/config)
else
	$(eval CONFIG_FILE = ${KUBECONFIG})
endif
	cp $(CONFIG_FILE) $(BUMPED_KUBECONFIG)

_login_with_devworkspace_sa:
	$(eval SA_TOKEN := $(shell $(K8S_CLI) get secrets -o=json -n $(NAMESPACE) | jq -r '[.items[] | select (.type == "kubernetes.io/service-account-token" and .metadata.annotations."kubernetes.io/service-account.name" == "default")][0].data.token' | base64 --decode ))
	echo "Logging as controller's SA in $(NAMESPACE)"
	oc login --token=$(SA_TOKEN) --kubeconfig=$(BUMPED_KUBECONFIG)

### run: Run against the configured Kubernetes cluster in ~/.kube/config
run: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa
	source $(RELATED_IMAGES_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=default \
		WATCH_NAMESPACE=$(NAMESPACE) \
		go run ./main.go


debug: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa
	source $(RELATED_IMAGES_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=default \
		WATCH_NAMESPACE=$(NAMESPACE) \
		dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --

### install_crds: Install CRDs into a cluster
install_crds: _kustomize _init_devworkspace_crds
	$(KUSTOMIZE) build config/crd | $(K8S_CLI) apply -f -

### install: Install controller in the configured Kubernetes cluster in ~/.kube/config
install: _print_vars _kustomize _init_devworkspace_crds _create_namespace deploy_registry
	mv config/cert-manager/kustomization.yaml config/cert-manager/kustomization.yaml.bak
	mv config/service-ca/kustomization.yaml config/service-ca/kustomization.yaml.bak
	mv config/base/config.properties config/base/config.properties.bak
	mv config/base/manager_image_patch.yaml config/base/manager_image_patch.yaml.bak

	envsubst < config/cert-manager/kustomization.yaml.bak > config/cert-manager/kustomization.yaml
	envsubst < config/service-ca/kustomization.yaml.bak > config/service-ca/kustomization.yaml
	envsubst < config/base/config.properties.bak > config/base/config.properties
	envsubst < config/base/manager_image_patch.yaml.bak > config/base/manager_image_patch.yaml
ifeq ($(PLATFORM),kubernetes)
	$(KUSTOMIZE) build config/cert-manager | $(K8S_CLI) apply -f - || true
else
	$(KUSTOMIZE) build config/service-ca | $(K8S_CLI) apply -f - || true
endif

	mv config/cert-manager/kustomization.yaml.bak config/cert-manager/kustomization.yaml
	mv config/service-ca/kustomization.yaml.bak config/service-ca/kustomization.yaml
	mv config/base/config.properties.bak config/base/config.properties
	mv config/base/manager_image_patch.yaml.bak config/base/manager_image_patch.yaml

### restart: Restart devworkspace-controller deployment
restart:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-controller-manager

### restart_webhook: Restart devworkspace-controller webhook deployment
restart_webhook:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-webhook-server

### uninstall: Remove controller resources from the cluster
uninstall: _kustomize
# It's safer to delete all workspaces before deleting the controller; otherwise we could
# leave workspaces in a hanging state if we add finalizers.
	$(K8S_CLI) delete devworkspaces.workspace.devfile.io --all-namespaces --all | true
	$(K8S_CLI) delete devworkspacetemplates.workspace.devfile.io --all-namespaces --all | true
# Have to wait for routings to be deleted in case there are finalizers
	$(K8S_CLI) delete workspaceroutings.controller.devfile.io --all-namespaces --all --wait | true
ifeq ($(PLATFORM),kubernetes)
	$(KUSTOMIZE) build config/cert-manager | $(K8S_CLI) delete --ignore-not-found -f -
else
	$(KUSTOMIZE) build config/service-ca | $(K8S_CLI) delete --ignore-not-found -f -
endif
	$(K8S_CLI) delete all -l "app.kubernetes.io/part-of=devworkspace-operator" --all-namespaces
	$(K8S_CLI) delete mutatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	$(K8S_CLI) delete validatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	$(K8S_CLI) delete namespace $(NAMESPACE) --ignore-not-found

### deploy_registry: Deploy plugin registry
deploy_registry: _print_vars _create_namespace
	$(K8S_CLI) apply -f config/registry/local -n $(NAMESPACE)
ifeq ($(PLATFORM),kubernetes)
	envsubst < config/registry/local/k8s/ingress.yaml | $(K8S_CLI) apply -n $(NAMESPACE) -f -
else
	$(K8S_CLI) apply -f config/registry/local/os -n $(NAMESPACE)
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
	addlicense -v -f license_header.txt $$(find . -name '*.go')
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
	GOFLAGS="" go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

### install_cert_manager: install Cert Mananger v1.0.4 on the cluster
install_cert_manager:
	kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.4/cert-manager.yaml

_kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	GOFLAGS="" go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
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
	@echo '    REGISTRY_ENABLED           - Whether the plugin registry should be deployed'
	@echo '    DEVWORKSPACE_API_VERSION   - Branch or tag of the github.com/devfile/api to depend on. Defaults to master'
