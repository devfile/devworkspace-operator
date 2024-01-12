#
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

### build_bundle_and_index: build and push DevWorkspace Operator OLM bundle and index images
build_bundle_and_index: _print_vars _check_skopeo_installed _check_opm_version
	./build/scripts/build_index_image.sh \
		--bundle-repo $${DWO_BUNDLE_IMG%%:*} \
		--bundle-tag $${DWO_BUNDLE_IMG##*:} \
		--index-image $(DWO_INDEX_IMG) \
		--container-tool $(DOCKER)

### register_catalogsource: create the catalogsource to make the operator be available on the marketplace
register_catalogsource: _check_skopeo_installed
	sed -e "s|quay.io/devfile/devworkspace-operator-index:next|$(DWO_INDEX_IMG)|g" ./catalog-source.yaml \
	  | oc apply -f -

### unregister_catalogsource: unregister the catalogsource and delete the imageContentSourcePolicy
unregister_catalogsource:
	oc delete catalogsource devworkspace-operator-catalog -n openshift-marketplace --ignore-not-found

_generate_olm_deployment_files:
	build/scripts/generate_deployment.sh --generate-olm

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
