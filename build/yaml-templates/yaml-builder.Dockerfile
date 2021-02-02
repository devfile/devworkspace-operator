FROM alpine AS builder

ARG DEVWORKSPACE_BRANCH=master
ARG NAMESPACE=devworkspace-controller
ARG IMG=quay.io/devfile/devworkspace-controller:next
ARG PULL_POLICY=Always
ARG DEFAULT_ROUTING=basic
ARG DEVWORKSPACE_API_VERSION=aeda60d4361911da85103f224644bfa792498499

WORKDIR /build

RUN apk add --no-cache \
    bash \
    coreutils \
    curl \
    gettext \
    git \
    jq \
    python3 \
    py-pip \
    tar \
    && pip install yq \
    && curl -sL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.9.2/kustomize_v3.9.2_linux_amd64.tar.gz" -o kustomize.tar.gz \
    && tar -xvf kustomize.tar.gz --directory /usr/bin

COPY ["prepare_templates.sh", "/"]
RUN /prepare_templates.sh

FROM alpine

COPY --from=builder "/build/devworkspace_operator_templates.tar.gz" /devworkspace_operator_templates.tar.gz
CMD echo "To extract yaml files from this repo, run:" &&\
    echo "    docker create --name builder <this-image>" &&\
    echo "    docker cp builder:/devworkspace_operator_templates.tar.gz ./devworkspace_operator_templates.tar.gz" &&\
    echo "    docker rm builder" &&\
    echo "    tar -xzf devworkspace_operator_templates.tar.gz"
