#!/bin/bash

set -e

usage () {
	echo "Usage:   $0 -c [/path/to/csv.yaml]"
	echo "Example: $0 -c devworkspace-operator.clusterserviceversion.yaml"
}

unset CSV_FILE
while [[ "$#" -gt 0 ]]; do
  case $1 in
    '-c') CSV_FILE="$2";shift 1;;
    '--help'|'-h') usage; exit;;
  esac
  shift 1
done

if [[ ! ${CSV_FILE} ]]; then usage; exit 1; fi

set_image_digest() {
  local JQ_FILTER=${1}
  local IMAGE=$(yq -r ${JQ_FILTER} "${CSV_FILE}")
  if [[ ! ${IMAGE} =~ sha256 ]]; then
    local DIGEST=$(skopeo inspect --tls-verify=false docker://${IMAGE} 2>/dev/null | jq -r '.Digest')
    local IMAGE_WITH_DIGEST=$(echo "${IMAGE}" | sed -e 's/^\(.*\):[^:]*$/\1/')@"${DIGEST}"
    yq -riY ''"${JQ_FILTER}"' = '"\"${IMAGE_WITH_DIGEST}\""'' ${CSV_FILE}

    echo "[INFO] Set image digests for ${JQ_FILTER}: ${IMAGE} => ${IMAGE_WITH_DIGEST}"
  fi
}

run() {
  i=0
  local CONTAINERS_LENGTH=$(yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers | length' ${CSV_FILE})
  while [ "${i}" -lt "${CONTAINERS_LENGTH}" ]; do
      # RELATED_IMAGE environment variables
      j=0
      local ENV_VARS_LENGTH=$(yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers['"${i}"'].env | length' ${CSV_FILE})
      while [ "${j}" -lt "${ENV_VARS_LENGTH}" ]; do
          ENV_NAME=$(yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers['"${i}"'].env['"${j}"'].name' ${CSV_FILE})
          if [[ ${ENV_NAME} =~ ^RELATED_IMAGE_ ]]; then
              set_image_digest ".spec.install.spec.deployments[0].spec.deploy/templates/components/csv/clusterserviceversion.yamltemplate.spec.containers[${i}].env[${j}].value"
          fi

          j=$((j+1))
      done

      # Container image
      set_image_digest ".spec.install.spec.deployments[0].spec.template.spec.containers[${i}].image"

      i=$((i+1))
  done

  # Related images
  i=0
  local RELATED_IMAGE_LENGTH=$(yq -r '.spec.relatedImages | length' ${CSV_FILE})
  while [ "${i}" -lt "${RELATED_IMAGE_LENGTH}" ]; do
      set_image_digest ".spec.relatedImages[${i}].image"
      i=$((i+1))
  done
}

run