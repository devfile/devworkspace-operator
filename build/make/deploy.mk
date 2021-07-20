
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

_create_namespace:
	$(K8S_CLI) create namespace $(NAMESPACE) || true

_gen_configuration_env:
	mkdir -p $(INTERNAL_TMP_DIR)
	cat deploy/templates/components/manager/manager.yaml \
		| yq -r \
			'.spec.template.spec.containers[]
				| select(.name=="devworkspace-controller")
				| .env[]
				| select(has("value"))
				| "export \(.name)=\"$${\(.name):-\(.value)}\""' \
		> $(CONTROLLER_ENV_FILE)
	echo "export RELATED_IMAGE_devworkspace_webhook_server=$(DWO_IMG)" >> $(CONTROLLER_ENV_FILE)
	echo "export WEBHOOK_SECRET_NAME=devworkspace-operator-webhook-cert" >> $(CONTROLLER_ENV_FILE)
	cat $(CONTROLLER_ENV_FILE)

_store_tls_cert:
	mkdir -p /tmp/k8s-webhook-server/serving-certs/
	$(K8S_CLI) get secret devworkspace-webhooks-tls -n $(NAMESPACE) -o json | jq -r '.data["tls.crt"]' | base64 -d > /tmp/k8s-webhook-server/serving-certs/tls.crt
	$(K8S_CLI) get secret devworkspace-webhooks-tls -n $(NAMESPACE) -o json | jq -r '.data["tls.key"]' | base64 -d > /tmp/k8s-webhook-server/serving-certs/tls.key

### install: Install controller in the configured Kubernetes cluster in ~/.kube/config
install: _check_cert_manager _print_vars _init_devworkspace_crds _create_namespace generate_deployment
ifeq ($(PLATFORM),kubernetes)
	$(K8S_CLI) apply -f deploy/current/kubernetes/combined.yaml
else
	$(K8S_CLI) apply -f deploy/current/openshift/combined.yaml
endif


### install_plugin_templates: Deploys the sample plugin templates to namespace devworkspace-plugins:
install_plugin_templates: _print_vars
	$(K8S_CLI) create namespace devworkspace-plugins || true
	$(K8S_CLI) apply -f samples/plugins -n devworkspace-plugins

### restart: Restarts the devworkspace-controller deployment
restart:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-controller-manager

### restart_webhook: Restarts the devworkspace-controller webhook deployment
restart_webhook:
	$(K8S_CLI) rollout restart -n $(NAMESPACE) deployment/devworkspace-webhook-server

### uninstall: Removes the controller resources from the cluster
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
ifneq ($(NAMESPACE),openshift-operators)
	$(K8S_CLI) delete namespace $(NAMESPACE) --ignore-not-found
endif

_check_cert_manager:
ifeq ($(PLATFORM),kubernetes)
	if ! ${K8S_CLI} api-versions | grep -q '^cert-manager.io/v1$$' ; then \
		echo "Cert-manager is required for deploying on Kubernetes. See 'make install_cert_manager'" ;\
		exit 1 ;\
	fi
endif

_login_with_devworkspace_sa:
	$(eval SA_TOKEN := $(shell $(K8S_CLI) get secrets -o=json -n $(NAMESPACE) | jq -r '[.items[] | select (.type == "kubernetes.io/service-account-token" and .metadata.annotations."kubernetes.io/service-account.name" == "$(DEVWORKSPACE_CTRL_SA)")][0].data.token' | base64 --decode ))
	echo "Logging as controller's SA in $(NAMESPACE)"
	oc login --token=$(SA_TOKEN) --kubeconfig=$(BUMPED_KUBECONFIG)

### install_cert_manager: Installs Cert Mananger v1.0.4 on the cluster
install_cert_manager:
	${K8S_CLI} apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.4/cert-manager.yaml

# it's easier to bump whole kubeconfig instead of grabbing cluster URL from the current context
_bump_kubeconfig:
	mkdir -p $(INTERNAL_TMP_DIR)
ifndef KUBECONFIG
	$(eval CONFIG_FILE = ${HOME}/.kube/config)
else
	$(eval CONFIG_FILE = ${KUBECONFIG})
endif
	cp $(CONFIG_FILE) $(BUMPED_KUBECONFIG)

### run: Runs against the configured Kubernetes cluster in ~/.kube/config
run: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa _store_tls_cert
	source $(CONTROLLER_ENV_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=$(DEVWORKSPACE_CTRL_SA) \
		WATCH_NAMESPACE=$(NAMESPACE) \
		go run ./main.go

### debug: Runs the controller locally with debugging enabled, watching cluster defined in ~/.kube/config
debug: _print_vars _gen_configuration_env _bump_kubeconfig _login_with_devworkspace_sa _store_tls_cert
	source $(CONTROLLER_ENV_FILE)
	export KUBECONFIG=$(BUMPED_KUBECONFIG)
	CONTROLLER_SERVICE_ACCOUNT_NAME=$(DEVWORKSPACE_CTRL_SA) \
		WATCH_NAMESPACE=$(NAMESPACE) \
		dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --
