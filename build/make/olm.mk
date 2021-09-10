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

### generate_olm_bundle: Generate yaml files for building an OLM bundle image
generate_olm_bundle_yaml: _check_operator_sdk_version _generate_olm_deployment_files
# Note: operator-sdk provides no way to specify output dir for bundle.Dockerfile so
# we have to move it manually
#
# `<&-` closes stdin to workaround cases when a tty is not allocated,
# so stdin behaves like a pipe, like in Github actions case.
# Operator SDK checks if stdin is an open pipe and assumes it's reading 
# from stdin in that case. To make Operator SDK work correctly here,
# we need to explicitly close stdin. 
# See issue: https://github.com/actions/runner/issues/241
	rm -rf deploy/bundle/manifests
	operator-sdk generate bundle \
		--deploy-dir deploy/deployment/olm \
		--output-dir deploy/bundle \
		--manifests \
		--channels fast \
		--metadata <&- && \
	mv bundle.Dockerfile build/
# Operator SDK v1.8.0 does not output webhooks in a stable order, so we have to sort the yaml files to avoid
# spurious changes. See issue https://github.com/operator-framework/operator-sdk/issues/5022
	yq -iY '.spec.webhookdefinitions |= sort' deploy/bundle/manifests/devworkspace-operator.clusterserviceversion.yaml
# OLM creates a configmap that contains the files in bundle when an operator is installed. Since the maximum size
# of a resource in etcd is 1MiB, we need to do a bit of squishing on our yaml files to get the total bundle under the
# 1MiB limit. This command puts all YAML strings on a single line, avoiding ~200KiB of newlines and indentation.
	find deploy/bundle/manifests -name '*.yaml' -exec yq --indentless -w 1000000000 -iY . {} \;

### build_bundle_image: build and push DevWorkspace Operator bundle image
build_bundle_image: _print_vars _check_operator_sdk_version
ifneq ($(INITIATOR),CI)
ifeq ($(DWO_BUNDLE_IMG),quay.io/devfile/devworkspace-operator-bundle:next)
	@echo -n "Are you sure you want to push $(DWO_BUNDLE_IMG)? [y/N] " && read ans && [ $${ans:-N} = y ]
endif
endif
	$(DOCKER) build . -t $(DWO_BUNDLE_IMG) -f build/bundle.Dockerfile
	$(DOCKER) push $(DWO_BUNDLE_IMG)

### build_index_image: build and push DevWorkspace Operator index image
build_index_image: _print_vars _check_skopeo_installed _check_opm_version
ifneq ($(INITIATOR),CI)
ifeq ($(DWO_INDEX_IMG),quay.io/devfile/devworkspace-operator-index:next)
	@echo -n "Are you sure you want to push $(DWO_INDEX_IMG)? [y/N] " && read ans && [ $${ans:-N} = y ]
endif
endif
	export BUNDLE_DIGEST=$$(skopeo inspect docker://$(DWO_BUNDLE_IMG) | jq -r '.Digest') ;\
	echo "$$BUNDLE_DIGEST" ;\
	export BUNDLE_IMG=$(DWO_BUNDLE_IMG) ;\
	export BUNDLE_IMG_DIGEST="$${BUNDLE_IMG%:*}@$${BUNDLE_DIGEST}" ;\
	opm index add \
		--bundles "$${BUNDLE_IMG_DIGEST}" \
		--tag $(DWO_INDEX_IMG) \
		--container-tool $(DOCKER)
	$(DOCKER) push $(DWO_INDEX_IMG)

export_manifests: _print_vars _check_opm_version
	rm -rf ./generated/exported-manifests
	# Export the bundles with the name web-terminal inside of $(DWO_INDEX_IMG)
	# This command basic exports the index back into the old format
	opm index export -c $(DOCKER) -f ./generated/exported-manifests -i $(DWO_INDEX_IMG)

### register_catalogsource: create the catalogsource to make the operator be available on the marketplace
register_catalogsource: _check_skopeo_installed
	INDEX_DIGEST=$$(skopeo inspect docker://$(DWO_INDEX_IMG) | jq -r '.Digest')
	INDEX_IMG=$(DWO_INDEX_IMG)
	INDEX_IMG_DIGEST="$${INDEX_IMG%:*}@$${INDEX_DIGEST}"

	# replace references of catalogsource img with your image
	sed -e "s|quay.io/devfile/devworkspace-operator-index:next|$${INDEX_IMG_DIGEST}|g" ./catalog-source.yaml \
		| oc apply -f -

### unregister_catalogsource: unregister the catalogsource and delete the imageContentSourcePolicy
unregister_catalogsource:
	oc delete catalogsource custom-devworkspace-operator-catalog -n openshift-marketplace --ignore-not-found

_generate_olm_deployment_files:
	deploy/generate-deployment.sh --generate-olm

_check_operator_sdk_version:
	if ! command -v operator-sdk &>/dev/null; then \
		echo "Operator SDK is required for this rule; see https://github.com/operator-framework/operator-sdk" ;\
		exit 1 ;\
	fi
	if [ "$$(operator-sdk version | grep -o 'operator-sdk version: [^ ,]*')" != 'operator-sdk version: "$(OPERATOR_SDK_VERSION)"' ]; then \
		echo "Operator SDK version $(OPERATOR_SDK_VERSION) is required." ;\
		exit 1 ;\
	fi

_check_opm_version:
	if ! command -v opm &>/dev/null; then \
		echo "The opm binary is required for this rule; see https://github.com/operator-framework/operator-registry" ;\
		exit 1 ;\
	elif [ "$$(opm version | grep -o 'OpmVersion:[^ ,]*')" != 'OpmVersion:"$(OPM_VERSION)"' ]; then \
		echo "opm version $(OPM_VERSION) is required." ;\
		exit 1 ;\
	fi

_check_skopeo_installed:
	if ! command -v skopeo &> /dev/null; then \
		echo "Skopeo is required for building and deploying bundle, but it is not installed." ;\
		exit 1
	fi
