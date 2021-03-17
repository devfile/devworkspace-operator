# Copyright (c) 2019-2021 Red Hat, Inc.
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
export DWO_IMG ?= quay.io/devfile/devworkspace-controller:next
export ROUTING_SUFFIX ?= 192.168.99.100.nip.io
export PULL_POLICY ?= Always
export DEFAULT_ROUTING ?= basic
export KUBECONFIG ?= ${HOME}/.kube/config
export DEVWORKSPACE_API_VERSION ?= 283b0c54946e9fea9872c25e1e086c303688f0e8

#internal params
DEVWORKSPACE_CTRL_SA=devworkspace-controller-serviceaccount
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

ifeq ($(shell $(K8S_CLI) api-resources --api-group='route.openshift.io'  2>&1 | grep -o routes),routes)
PLATFORM := openshift
else
PLATFORM := kubernetes
endif

# minikube handling
ifeq ($(shell $(K8S_CLI) config current-context 2>&1),minikube)
# check ingress addon is enabled
ifeq ($(shell minikube addons list -o json | jq -r .ingress.Status), disabled)
$(error ingress addon should be enabled on top of minikube)
endif
export ROUTING_SUFFIX := $(shell minikube ip).nip.io
endif

# Bootstrapped by Operator-SDK v1.1.0
# Current Operator version
VERSION ?= 0.0.1
OPERATOR_SDK_VERSION = v1.1.0
# Default bundle image tag
DWO_BUNDLE_IMG ?= controller-bundle:$(VERSION)
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
	@echo "    DWO_IMG=$(DWO_IMG)"
	@echo "    PULL_POLICY=$(PULL_POLICY)"
	@echo "    ROUTING_SUFFIX=$(ROUTING_SUFFIX)"
	@echo "    DEFAULT_ROUTING=$(DEFAULT_ROUTING)"
	@echo "    DEVWORKSPACE_API_VERSION=$(DEVWORKSPACE_API_VERSION)"

_create_namespace:
	$(K8S_CLI) create namespace $(NAMESPACE) || true

_gen_configuration_env:
	mkdir -p $(INTERNAL_TMP_DIR)
	echo "export RELATED_IMAGE_devworkspace_webhook_server=$(DWO_IMG)" > $(RELATED_IMAGES_FILE)
ifeq ($(PLATFORM),kubernetes)
	echo "export WEBHOOK_SECRET_NAME=devworkspace-operator-webhook-cert" >> $(RELATED_IMAGES_FILE)
endif
	cat ./deploy/templates/components/manager/manager.yaml \
		| yq -r \
			'.spec.template.spec.containers[]?.env[] | select(.name | startswith("RELATED_IMAGE")) | "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		>> $(RELATED_IMAGES_FILE)
	cat $(RELATED_IMAGES_FILE)

##### Rules for dealing with devfile/api
### update_devworkspace_api: update version of devworkspace crds in go.mod
update_devworkspace_api:
	go mod edit --require github.com/devfile/api/v2@$(DEVWORKSPACE_API_VERSION)
	go mod download
	go mod tidy

_init_devworkspace_crds:
	./update_devworkspace_crds.sh --init --api-version $(DEVWORKSPACE_API_VERSION)

### update_devworkspace_crds: pull latest devworkspace CRDs to ./devworkspace-crds. Note: pulls master branch
update_devworkspace_crds:
	./update_devworkspace_crds.sh --api-version $(DEVWORKSPACE_API_VERSION)
###### End rules for dealing with devfile/api

### test: Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/bin/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test $(shell go list ./... | grep -v test/e2e) -coverprofile cover.out

### test_e2e: runs e2e test on the cluster set in context. DevWorkspace Operator must be already deployed
test_e2e:
	CGO_ENABLED=0 go test -v -c -o bin/devworkspace-controller-e2e ./test/e2e/cmd/workspaces_test.go
	./bin/devworkspace-controller-e2e -ginkgo.failFast

### test_e2e_debug: runs e2e test in debug mode, so it's possible to connect to execution via remote debugger
test_e2e_debug:
	dlv test --listen=:2345 --headless=true --api-version=2 ./test/e2e/cmd/workspaces_test.go -- --ginkgo.failFast

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
	$(eval SA_TOKEN := $(shell $(K8S_CLI) get secrets -o=json -n $(NAMESPACE) | jq -r '[.items[] | select (.type == "kubernetes.io/service-account-token" and .metadata.annotations."kubernetes.io/service-account.name" == "$(DEVWORKSPACE_CTRL_SA)")][0].data.token' | base64 --decode ))
	echo "Logging as controller's SA in $(NAMESPACE)"
	oc login --token=$(SA_TOKEN) --kubeconfig=$(BUMPED_KUBECONFIG)

### run: Run against the configured Kubernetes cluster in ~/.kube/config
run: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa
	source $(RELATED_IMAGES_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=$(DEVWORKSPACE_CTRL_SA) \
		WATCH_NAMESPACE=$(NAMESPACE) \
		go run ./main.go

### debug: Run controller locally with debugging enabled, watching cluster defined in ~/.kube/config
debug: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa
	source $(RELATED_IMAGES_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=$(DEVWORKSPACE_CTRL_SA) \
		WATCH_NAMESPACE=$(NAMESPACE) \
		dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --

### install: Install controller in the configured Kubernetes cluster in ~/.kube/config
install: _check_cert_manager _print_vars _init_devworkspace_crds _create_namespace generate_deployment
ifeq ($(PLATFORM),kubernetes)
	$(K8S_CLI) apply -f deploy/current/kubernetes/combined.yaml
else
	$(K8S_CLI) apply -f deploy/current/openshift/combined.yaml
endif

### generate_deployment: Generate files used for deployment from kustomize templates, using environment variables
generate_deployment:
	deploy/generate-deployment.sh

### generate_default_deployment: Generate files used for deployment from kustomize templates with default values
generate_default_deployment:
	deploy/generate-deployment.sh --use-defaults

### install_plugin_templates: Deploy sample plugin templates to namespace devworkspace-plugins:
install_plugin_templates: _print_vars
	$(K8S_CLI) create namespace devworkspace-plugins || true
	$(K8S_CLI) apply -f samples/plugins -n devworkspace-plugins

### restart: Restart devworkspace-controller deployment
restart:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-controller-manager

### restart_webhook: Restart devworkspace-controller webhook deployment
restart_webhook:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-webhook-server

### uninstall: Remove controller resources from the cluster
uninstall: generate_deployment
# It's safer to delete all workspaces before deleting the controller; otherwise we could
# leave workspaces in a hanging state if we add finalizers.
	$(K8S_CLI) delete devworkspaces.workspace.devfile.io --all-namespaces --all --wait || true
	$(K8S_CLI) delete devworkspacetemplates.workspace.devfile.io --all-namespaces --all || true
	$(K8S_CLI) delete devworkspaceroutings.controller.devfile.io --all-namespaces --all --wait || true

ifeq ($(PLATFORM),kubernetes)
	$(K8S_CLI) delete --ignore-not-found -f deploy/current/kubernetes/combined.yaml || true
else
	$(K8S_CLI) delete --ignore-not-found -f deploy/current/openshift/combined.yaml || true
endif

	$(K8S_CLI) delete all -l "app.kubernetes.io/part-of=devworkspace-operator" --all-namespaces
	$(K8S_CLI) delete mutatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	$(K8S_CLI) delete validatingwebhookconfigurations.admissionregistration.k8s.io controller.devfile.io --ignore-not-found
	$(K8S_CLI) delete namespace $(NAMESPACE) --ignore-not-found

### manifests: Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=role webhook paths="./..." \
			output:crd:artifacts:config=deploy/templates/crd/bases \
			output:rbac:artifacts:config=deploy/templates/components/rbac
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

### check_fmt: check formatting on files in repo
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
	docker build . -t ${DWO_IMG} -f build/Dockerfile

### docker-push: Push the controller image
docker-push:
ifneq ($(INITIATOR),CI)
ifeq ($(DWO_IMG),quay.io/devfile/devworkspace-controller:next)
	@echo -n "Are you sure we want to push $(DWO_IMG)? [y/N] " && read ans && [ $${ans:-N} = y ]
endif
endif
	docker push ${DWO_IMG}

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

_operator_sdk:
	@{ \
		if ! command -v operator-sdk &> /dev/null; then \
			echo 'operator-sdk $(OPERATOR_SDK_VERSION) is expected to be used for this target but it is not installed' ;\
			exit 1 ;\
		else \
			SDK_VER=$$(operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2) && \
			if [ "$${SDK_VER}" != $(OPERATOR_SDK_VERSION) ]; then \
				echo "WARN: operator-sdk $(OPERATOR_SDK_VERSION) is expected to be used for this target but $${SDK_VER} found" \
				echo "WARN: Please use the recommended operator-sdk if you face any issue" \
			fi \
		fi \
	}
ifneq ($(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2),$(OPERATOR_SDK_VERSION))
	@echo 'WARN: operator-sdk $(OPERATOR_SDK_VERSION) is expected to be used for this target but $(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2) found.'
	@echo 'WARN: Please use the recommended operator-sdk if you face any issue.'
endif

_check_cert_manager:
ifeq ($(PLATFORM),kubernetes)
	if ! ${K8S_CLI} api-versions | grep -q '^cert-manager.io/v1$$' ; then \
		echo "Cert-manager is required for deploying on Kubernetes. See 'make install_cert_manager'" ;\
		exit 1 ;\
	fi
endif

.PHONY: help
### help: print this message
help: Makefile
	@echo 'Available rules:'
	@sed -n 's/^### /    /p' $< | awk 'BEGIN { FS=":" } { printf "%-30s -%s\n", $$1, $$2 }'
	@echo ''
	@echo 'Supported environment variables:'
	@echo '    DWO_IMG                    - Image used for controller'
	@echo '    NAMESPACE                  - Namespace to use for deploying controller'
	@echo '    KUBECONFIG                 - Kubeconfig which should be used for accessing to the cluster. Currently is: $(KUBECONFIG)'
	@echo '    ROUTING_SUFFIX             - Cluster routing suffix (e.g. $$(minikube ip).nip.io, apps-crc.testing)'
	@echo '    PULL_POLICY                - Image pull policy for controller'
	@echo '    DEVWORKSPACE_API_VERSION   - Branch or tag of the github.com/devfile/api to depend on. Defaults to master'
