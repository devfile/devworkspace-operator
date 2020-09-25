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

OPERATOR_SDK_VERSION = v0.17.0
NAMESPACE ?= devworkspace-controller
IMG ?= quay.io/devfile/devworkspace-controller:next
TOOL ?= oc
ROUTING_SUFFIX ?= 192.168.99.100.nip.io
PULL_POLICY ?= Always
WEBHOOK_ENABLED ?= true
DEFAULT_ROUTING ?= basic
ADMIN_CTX ?= ""
REGISTRY_ENABLED ?= true
DEVWORKSPACE_API_VERSION ?= v1alpha1

#internal params
INTERNAL_TMP_DIR=/tmp/devworkspace-controller
BUMPED_KUBECONFIG=$(INTERNAL_TMP_DIR)/kubeconfig
RELATED_IMAGES_FILE=$(INTERNAL_TMP_DIR)/environment

# minikube handling
ifeq ($(shell $(TOOL) config current-context),minikube)
	ROUTING_SUFFIX := $(shell minikube ip).nip.io
	TOOL := kubectl
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
	@echo "    TOOL=$(TOOL)"

_set_ctx:
ifneq ($(ADMIN_CTX),"")
	$(eval CURRENT_CTX := $(shell $(TOOL) config current-context))
	@echo "Switching current ctx to $(ADMIN_CTX) from $(CURRENT_CTX)"
	$(TOOL) config use-context $(ADMIN_CTX)
endif

_reset_ctx:
ifneq ($(ADMIN_CTX),"")
	@echo "Restoring the current context to $(CURRENT_CTX)"
	$(TOOL) config use-context $(CURRENT_CTX)
endif

_create_namespace:
	$(TOOL) create namespace $(NAMESPACE) || true

_deploy_registry:
ifeq ($(REGISTRY_ENABLED),true)
	$(TOOL) apply -f ./deploy/registry/local -n $(NAMESPACE)
ifeq ($(TOOL),oc)
	$(TOOL) apply -f ./deploy/registry/local/os -n $(NAMESPACE)
else
	sed -i.bak -e  "s|192.168.99.100.nip.io|$(ROUTING_SUFFIX)|g" ./deploy/registry/local/k8s/ingress.yaml
	$(TOOL) apply -f ./deploy/registry/local/k8s -n $(NAMESPACE)
	sed -i.bak -e "s|$(ROUTING_SUFFIX)|192.168.99.100.nip.io|g" ./deploy/registry/local/k8s/ingress.yaml
	rm ./deploy/registry/local/k8s/ingress.yaml.bak
endif
endif

_set_registry_url:
ifeq ($(TOOL),oc)
	$(eval PLUGIN_REGISTRY_HOST := $(shell $(TOOL) get route che-plugin-registry -n $(NAMESPACE) -o jsonpath='{.spec.host}' || echo ""))
else
	$(eval PLUGIN_REGISTRY_HOST := $(shell $(TOOL) get ingress che-plugin-registry -n $(NAMESPACE) -o jsonpath='{.spec.rules[0].host}' || echo ""))
endif

# -i.bak is needed for compatibility between OS X and Linux versions of sed
_update_yamls: _set_registry_url
	sed -i.bak -e "s|controller.plugin_registry.url: .*|controller.plugin_registry.url: http://$(PLUGIN_REGISTRY_HOST)|g" ./deploy/controller_config.yaml
	sed -i.bak -e 's|controller.webhooks.enabled: .*|controller.webhooks.enabled: "$(WEBHOOK_ENABLED)"|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.default_routing_class: .*|devworkspace.default_routing_class: "$(DEFAULT_ROUTING)"|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.routing.cluster_host_suffix: .*|devworkspace.routing.cluster_host_suffix: $(ROUTING_SUFFIX)|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.sidecar.image_pull_policy: .*|devworkspace.sidecar.image_pull_policy: $(PULL_POLICY)|g' ./deploy/controller_config.yaml
	rm ./deploy/controller_config.yaml.bak
	sed -i.bak -e 's|namespace: $${NAMESPACE}|namespace: $(NAMESPACE)|' ./deploy/role_binding.yaml
	sed -i.bak -e "s|image: .*|image: $(IMG)|g" ./deploy/controller.yaml
	sed -i.bak -e "s|value: \"quay.io/devfile/devworkspace-controller:next\"|value: $(IMG)|g" ./deploy/controller.yaml
	sed -i.bak -e "s|imagePullPolicy: Always|imagePullPolicy: $(PULL_POLICY)|g" ./deploy/controller.yaml
	sed -i.bak -e "s|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: '$$(date +%Y-%m-%dT%H:%M:%S%z)'|g" ./deploy/controller.yaml

_reset_yamls: _set_registry_url
	sed -i.bak -e "s|http://$(PLUGIN_REGISTRY_HOST)|http://che-plugin-registry.192.168.99.100.nip.io/v3|g" ./deploy/controller_config.yaml
	sed -i.bak -e 's|controller.webhooks.enabled: .*|controller.webhooks.enabled: "true"|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.default_routing_class: .*|devworkspace.default_routing_class: "basic"|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.routing.cluster_host_suffix: .*|devworkspace.routing.cluster_host_suffix: 192.168.99.100.nip.io|g' ./deploy/controller_config.yaml
	sed -i.bak -e 's|devworkspace.sidecar.image_pull_policy: .*|devworkspace.sidecar.image_pull_policy: Always|g' ./deploy/controller_config.yaml
	rm ./deploy/controller_config.yaml.bak
	mv ./deploy/role_binding.yaml.bak ./deploy/role_binding.yaml
	sed -i.bak -e "s|image: .*|image: quay.io/devfile/devworkspace-controller:next|g" ./deploy/controller.yaml
	# webhook server related image
	sed -i.bak -e "s|value: $(IMG)|value: \"quay.io/devfile/devworkspace-controller:next\"|g" ./deploy/controller.yaml
	sed -i.bak -e "s|imagePullPolicy: .*|imagePullPolicy: Always|g" ./deploy/controller.yaml
	sed -i.bak -e 's|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: ""|g' ./deploy/controller.yaml
	rm ./deploy/controller.yaml.bak

_update_crds: update_devworkspace_crds
	$(TOOL) apply -f ./deploy/crds
	$(TOOL) apply -f ./devworkspace-crds/deploy/crds

_update_controller_deps:
	$(TOOL) apply -f ./deploy/controller_config.yaml -n $(NAMESPACE)
	$(TOOL) apply -f ./deploy/service_account.yaml -n $(NAMESPACE)
	$(TOOL) apply -f ./deploy/role.yaml -n $(NAMESPACE)
	$(TOOL) apply -f ./deploy/role_binding.yaml -n $(NAMESPACE)

_apply_controller_cfg:
	$(TOOL) apply -f ./deploy -n $(NAMESPACE)
	$(TOOL) apply -f ./deploy/controller.yaml -n $(NAMESPACE)

_do_restart:
ifeq ($(TOOL),oc)
	oc patch deployment/devworkspace-controller \
		-n $(NAMESPACE) \
		--patch "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"$$(date --iso-8601=seconds)\"}}}}}"
else
	kubectl rollout restart -n $(NAMESPACE) deployment/devworkspace-controller
endif

_do_restart_webhook_server:
ifeq ($(TOOL),oc)
	oc patch deployment/devworkspace-webhook-server \
		-n $(NAMESPACE) \
		--patch "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"$$(date --iso-8601=seconds)\"}}}}}"
else
	kubectl rollout restart -n $(NAMESPACE) deployment/devworkspace-webhook-server
endif

_do_uninstall:
# It's safer to delete all workspaces before deleting the controller; otherwise we could
# leave workspaces in a hanging state if we add finalizers.
ifneq ($(shell command -v kubectl 2> /dev/null),)
	kubectl delete devworkspaces.workspace.devfile.io --all-namespaces --all | true
	kubectl delete devworkspacetemplates.workspace.devfile.io --all-namespaces --all | true
# Have to wait for routings to be deleted in case there are finalizers
	kubectl delete workspaceroutings.controller.devfile.io --all-namespaces --all --wait | true
else
ifneq ($(TOOL) get devworkspaces.workspace.devfile.io --all-namespaces,"No resources found.")
	$(info To automatically remove all workspaces when uninstalling, ensure kubectl is installed)
	$(error Cannot uninstall operator, workspaces still running. Delete all workspaces and workspaceroutings before proceeding)
endif
endif
	$(TOOL) delete -f ./deploy -n $(NAMESPACE) --ignore-not-found=true
	$(TOOL) delete namespace $(NAMESPACE) --ignore-not-found=true
	$(TOOL) delete mutatingwebhookconfigurations controller.devfile.io --ignore-not-found=true
	$(TOOL) delete validatingwebhookconfigurations controller.devfile.io --ignore-not-found=true
	$(TOOL) delete clusterrole devworkspace-webhook-server --ignore-not-found=true
	$(TOOL) delete clusterrolebinding devworkspace-webhook-server --ignore-not-found=true
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io workspaceroutings.controller.devfile.io --ignore-not-found=true
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io components.controller.devfile.io --ignore-not-found=true
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io devworkspaces.workspace.devfile.io --ignore-not-found=true
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io devworkspacetemplates.workspace.devfile.io --ignore-not-found=true

_do_e2e_test:
	CGO_ENABLED=0 go test -v -c -o bin/devworkspace-controller-e2e ./test/e2e/cmd/workspaces_test.go
	./bin/devworkspace-controller-e2e

# it's easier to bump whole kubeconfig instead of grabbing cluster URL from the current context
_bump_kubeconfig:
	@mkdir -p $(INTERNAL_TMP_DIR)
ifndef KUBECONFIG
	$(eval CONFIG_FILE = ${HOME}/.kube/config)
else
	$(eval CONFIG_FILE = ${KUBECONFIG})
endif
	cp $(CONFIG_FILE) $(BUMPED_KUBECONFIG)

_generate_related_images_env:
	@mkdir -p $(INTERNAL_TMP_DIR)
	cat ./deploy/controller.yaml \
		| yq -r \
			'.spec.template.spec.containers[].env[]
				| select(.name | startswith("RELATED_IMAGE"))
				| "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		> $(RELATED_IMAGES_FILE)
	cat $(RELATED_IMAGES_FILE)

_login_with_devworkspace_sa:
	@$(eval SA_TOKEN := $(shell $(TOOL) get secrets -o=json -n $(NAMESPACE) | jq -r '[.items[] | select (.type == "kubernetes.io/service-account-token" and .metadata.annotations."kubernetes.io/service-account.name" == "devworkspace-controller")][0].data.token' | base64 --decode ))
	echo "Logging as devworkspace controller SA"
	oc login --token=$(SA_TOKEN) --kubeconfig=$(BUMPED_KUBECONFIG)

### docker: build and push docker image
docker: _print_vars
	docker build -t $(IMG) -f ./build/Dockerfile .
	docker push $(IMG)

### info: display info
info: _print_vars

### deploy: deploy controller to cluster
deploy: _print_vars _set_ctx _create_namespace _deploy_registry _update_yamls _update_crds _apply_controller_cfg _reset_yamls _reset_ctx

### restart: restart cluster controller deployment
restart: _set_ctx _do_restart _reset_ctx

### restart: restart cluster controller deployment
restart_webhook_server: _set_ctx _do_restart_webhook_server _reset_ctx

### rollout: rebuild and push docker image and restart cluster deployment
rollout: docker restart

### update_cfg: configures already deployed controller according to set env variables
update_cfg: _print_vars _set_ctx _update_yamls _apply_controller_cfg _reset_yamls _reset_ctx

### update_crds: update custom resource definitions on cluster
update_crds: _set_ctx _update_crds _reset_ctx

### uninstall: remove namespace and all CRDs from cluster
uninstall: _set_ctx _do_uninstall _reset_ctx

### update_devworkspace_api: update version of devworkspace crds in go.mod
update_devworkspace_api:
	go mod edit --require github.com/devfile/api@$(DEVWORKSPACE_API_VERSION)
	go mod download
	go mod tidy

### update_devworkspace_crds: pull latest devworkspace CRDs to ./devworkspace-crds. Note: pulls master branch
update_devworkspace_crds:
	./update_devworkspace_crds.sh $(DEVWORKSPACE_API_VERSION)


### local: set up cluster for local development
local: _print_vars _set_ctx _create_namespace _deploy_registry _set_registry_url _update_yamls _update_crds _update_controller_deps _reset_yamls _reset_ctx

### generate: generates CRDs and Kubernetes code for custom resource
generate:
ifeq ($(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2),$(OPERATOR_SDK_VERSION))
	operator-sdk generate k8s
	operator-sdk generate crds
	patch/patch_crds.sh
else
	$(error operator-sdk $(OPERATOR_SDK_VERSION) is expected to be used during CRDs and k8s objects generating while $(shell operator-sdk version | cut -d , -f 1 | cut -d : -f 2 | cut -d \" -f 2) found)
endif

### start_local: start local instance of controller using operator-sdk
start_local: _bump_kubeconfig _generate_related_images_env _login_with_devworkspace_sa
	@source $(RELATED_IMAGES_FILE)
ifeq ($(WEBHOOK_ENABLED),true)
	#in cluster mode it comes from Deployment env var
	export RELATED_IMAGE_devworkspace_webhook_server=$(IMG)
	#in cluster mode it comes from configured SA propogated via env var
	export CONTROLLER_SERVICE_ACCOUNT_NAME=devworkspace-controller
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
endif
	operator-sdk run --local --watch-namespace $(NAMESPACE) 2>&1 | grep --color=always -E '"msg":"[^"]*"|$$'

### start_local_debug: start local instance of controller with debugging enabled
start_local_debug: _bump_kubeconfig _generate_related_images_env _login_with_devworkspace_sa
	@source $(RELATED_IMAGES_FILE)
ifeq ($(WEBHOOK_ENABLED),true)
	#in cluster mode it comes from Deployment env var
	export RELATED_IMAGE_devworkspace_webhook_server=$(IMG)
	#in cluster mode it comes from configured SA propogated via env var
	export CONTROLLER_SERVICE_ACCOUNT_NAME=devworkspace-controller
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
endif
	operator-sdk run --local --watch-namespace $(NAMESPACE) --enable-delve 2>&1 | grep --color=always -E '"msg":"[^"]*"|$$'

.PHONY: test
### test: run unit tests
test:
	go test $(shell go list ./... | grep -v test/e2e)

### test_e2e: runs e2e test on the cluster set in context. Includes deploying devworkspace-controller, run test workspace, uninstall devworkspace-controller
test_e2e: _print_vars _set_ctx _update_yamls update_devworkspace_crds _do_e2e_test _reset_yamls _reset_ctx

### fmt: format all go files in repository
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

.PHONY: help
### help: print this message
help: Makefile
	@echo 'Available rules:'
	@sed -n 's/^### /    /p' $< | awk 'BEGIN { FS=":" } { printf "%-30s -%s\n", $$1, $$2 }'
	@echo ''
	@echo 'Supported environment variables:'
	@echo '    IMG                        - Image used for controller'
	@echo '    NAMESPACE                  - Namespace to use for deploying controller'
	@echo '    TOOL                       - CLI tool for interfacing with the cluster: kubectl or oc; if oc is used, deployment is tailored to OpenShift, otherwise Kubernetes'
	@echo '    ROUTING_SUFFIX             - Cluster routing suffix (e.g. $$(minikube ip).nip.io, apps-crc.testing)'
	@echo '    PULL_POLICY                - Image pull policy for controller'
	@echo '    WEBHOOK_ENABLED            - Whether webhooks should be enabled in the deployment'
	@echo '    ADMIN_CTX                  - Kubectx entry that should be used during work with cluster. The current will be used if omitted'
	@echo '    REGISTRY_ENABLED           - Whether the plugin registry should be deployed'
	@echo '    DEVWORKSPACE_API_VERSION   - Branch or tag of the github.com/devfile/api to depend on. Defaults to master'
