NAMESPACE = che-workspace-controller
IMG ?= quay.io/che-incubator/che-workspace-controller:nightly
TOOL ?= oc
CLUSTER_IP ?= 192.168.99.100
PULL_POLICY ?= Always
WEBHOOK_ENABLED ?= false
DEFAULT_ROUTING ?= basic
ADMIN_CTX ?= ""

all: help

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
	$(TOOL) apply -f ./deploy/registry/local
ifeq ($(TOOL),oc)
	$(TOOL) apply -f ./deploy/registry/local/os
else
	sed -i "s|192.168.99.100|$(CLUSTER_IP)|g" ./deploy/registry/local/k8s/ingress.yaml
	$(TOOL) apply -f ./deploy/registry/local/k8s
endif

_set_registry_url:
ifeq ($(TOOL),oc)
	$(eval PLUGIN_REGISTRY_HOST := $(shell $(TOOL) get route che-plugin-registry -n $(NAMESPACE) -o jsonpath='{.spec.host}' || echo ""))
else
	$(eval PLUGIN_REGISTRY_HOST := $(shell $(TOOL) get ingress che-plugin-registry -n $(NAMESPACE) -o jsonpath='{.spec.rules[0].host}' || echo ""))
endif

_update_yamls: _set_registry_url
	sed -i "s|plugin.registry.url: .*|plugin.registry.url: http://$(PLUGIN_REGISTRY_HOST)|g" ./deploy/controller_config.yaml
	sed -i 's|che.webhooks.enabled: .*|che.webhooks.enabled: "$(WEBHOOK_ENABLED)"|g' ./deploy/controller_config.yaml
	sed -i 's|che.default_routing_class: .*|che.default_routing_class: "$(DEFAULT_ROUTING)"|g' ./deploy/controller_config.yaml
ifeq ($(TOOL),oc)
	sed -i "s|image: .*|image: $(IMG)|g" ./deploy/os/controller.yaml
	sed -i "s|imagePullPolicy: Always|imagePullPolicy: $(PULL_POLICY)|g" ./deploy/os/controller.yaml
	sed -i "s|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: '$$(date --iso-8601=seconds)'|g" ./deploy/os/controller.yaml
else
	sed -i "s|image: .*|image: $(IMG)|g" ./deploy/k8s/controller.yaml
	sed -i "s|imagePullPolicy: Always|imagePullPolicy: $(PULL_POLICY)|g" ./deploy/k8s/controller.yaml
	sed -i "s|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: '$$(date --iso-8601=seconds)'|g" ./deploy/k8s/controller.yaml
endif

_reset_yamls: _set_registry_url
	sed -i "s|http://$(PLUGIN_REGISTRY_HOST)|http://che-plugin-registry.192.168.99.100.nip.io/v3|g" ./deploy/controller_config.yaml
	sed -i 's|che.webhooks.enabled: .*|che.webhooks.enabled: "false"|g' ./deploy/controller_config.yaml
	sed -i 's|che.default_routing_class: .*|che.default_routing_class: "basic"|g' ./deploy/controller_config.yaml
ifeq ($(TOOL),oc)
	sed -i "s|image: $(IMG)|image: quay.io/che-incubator/che-workspace-controller:nightly|g" ./deploy/os/controller.yaml
	sed -i "s|imagePullPolicy: $(PULL_POLICY)|imagePullPolicy: Always|g" ./deploy/os/controller.yaml
	sed -i 's|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: ""|g' ./deploy/os/controller.yaml
else
	sed -i "s|image: $(IMG)|image: quay.io/che-incubator/che-workspace-controller:nightly|g" ./deploy/k8s/controller.yaml
	sed -i "s|imagePullPolicy: $(PULL_POLICY)|imagePullPolicy: Always|g" ./deploy/k8s/controller.yaml
	sed -i 's|kubectl.kubernetes.io/restartedAt: .*|kubectl.kubernetes.io/restartedAt: ""|g' ./deploy/k8s/controller.yaml
endif

_update_crds:
	$(TOOL) apply -f ./deploy/crds

_update_controller_configmap:
	$(TOOL) apply -f ./deploy/controller_config.yaml

_apply_controller_cfg:
	$(TOOL) apply -f ./deploy
ifeq ($(TOOL),oc)
	$(TOOL) apply -f ./deploy/os/
else
	$(TOOL) apply -f ./deploy/k8s/
endif

_do_restart:
ifeq ($(TOOL),oc)
	oc patch deployment/che-workspace-controller \
		-n che-workspace-controller \
		--patch "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kubectl.kubernetes.io/restartedAt\":\"$$(date --iso-8601=seconds)\"}}}}}"
else
	kubectl rollout restart -n $(NAMESPACE) che-workspace-controller
endif

_do_uninstall:
# It's safer to delete all workspaces before deleting the controller; otherwise we could
# leave workspaces in a hanging state if we add finalizers.
ifneq ($(shell command -v kubectl 2> /dev/null),)
	kubectl delete workspaces.workspace.che.eclipse.org --all-namespaces --all
else
	$(info WARN: kubectl is not installed: unable to delete all workspaces)
endif
	$(TOOL) delete namespace $(NAMESPACE)
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io workspaceroutings.workspace.che.eclipse.org
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io components.workspace.che.eclipse.org
	$(TOOL) delete customresourcedefinitions.apiextensions.k8s.io workspaces.workspace.che.eclipse.org

### docker: build and push docker image
docker:
	docker build -t $(IMG) -f ./build/Dockerfile .
	docker push $(IMG)

### webhook: generate certificates for webhooks and deploy to cluster; no-op if webhooks are disabled or running on OpenShift
webhook:
ifeq ($(WEBHOOK_ENABLED),true)
ifeq ($(TOOL),kubectl)
	./deploy/webhook-server-certs/deploy-webhook-server-certs.sh kubectl
endif
else
	@echo "Webhooks disabled, skipping certificate generation"
endif

### deploy: deploy controller to cluster
deploy: _set_ctx _create_namespace _deploy_registry _update_yamls _update_crds webhook _apply_controller_cfg _reset_yamls _reset_ctx

### restart: restart cluster controller deployment
restart: _set_ctx _do_restart _reset_ctx

### rollout: rebuild and push docker image and restart cluster deployment
rollout: docker restart

### update_cfg: configures already deployed controller according to set env variables
update_cfg: _set_ctx _update_yamls _apply_controller_cfg _reset_yamls _reset_ctx

### update_crds: update custom resource definitions on cluster
update_crds: _set_ctx _update_crds _reset_ctx

### uninstall: remove namespace and all CRDs from cluster
uninstall: _set_ctx _do_uninstall _reset_ctx

### local: set up cluster for local development
local: _set_ctx _create_namespace _deploy_registry _set_registry_url _update_yamls _update_crds _update_controller_configmap _reset_yamls _reset_ctx

### start_local: start local instance of controller using operator-sdk
start_local:
	operator-sdk up local --namespace $(NAMESPACE) 2>&1 | grep --color=always -E '"msg":"[^"]*"|$$'

### start_local_debug: start local instance of controller with debugging enabled
start_local_debug:
	operator-sdk up local --namespace $(NAMESPACE) --enable-delve 2>&1 | grep --color=always -E '"msg":"[^"]*"|$$'

### fmt: format all go files in repository
fmt:
	go fmt -x ./...

.PHONY: help
### help: print this message
help: Makefile
	@echo "Available rules:"
	@sed -n 's/^### /    /p' $< | awk 'BEGIN { FS=":" } { printf "%-22s -%s\n", $$1, $$2 }'
	@echo ""
	@echo "Supported environment variables:"
	@echo "    IMG                - Image used for controller"
	@echo "    NAMESPACE          - Namespace to use for deploying controller"
	@echo "    TOOL               - CLI tool for interfacing with the cluster: kubectl or oc; if oc is used, deployment is tailored to OpenShift, otherwise Kubernetes"
	@echo "    CLUSTER_IP         - For Kubernetes only, the ip address of the cluster (minikube ip)"
	@echo "    PULL_POLICY        - Image pull policy for controller"
	@echo "    WEBHOOK_ENABLED    - Whether webhooks should be enabled in the deployment"
	@echo  "    ADMIN_CTX          - Kubectx entry that should be used during work with cluster. The current will be used if omitted"
