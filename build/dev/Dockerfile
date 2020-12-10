FROM quay.io/eclipse/che-golang-1.14:nightly

USER 0

RUN apt-get update && apt-get install -y \
    make \
    git \
    vim nano \
    jq \
    gettext-base \
    python3-pip \
    # podman \ # TODO: podman not available in golang:1.14-stretch which is upstream for quay.io/eclipse/che-golang-1.14:nightly
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install --user yq

RUN mkdir "${HOME}/bin" \
    && curl -LO "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod 770 ./kubectl && mv ./kubectl ${HOME}/bin/kubectl \
    && curl -LO "https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/stable/openshift-client-linux.tar.gz" \
    && tar -xf openshift-client-linux.tar.gz && mv ./oc ${HOME}/bin/oc \
    && curl -LO "https://github.com/operator-framework/operator-sdk/releases/download/v1.1.0/operator-sdk-v1.1.0-x86_64-linux-gnu" \
    && chmod 770 operator-sdk-v1.1.0-x86_64-linux-gnu && mv ./operator-sdk-v1.1.0-x86_64-linux-gnu ${HOME}/bin/operator-sdk \
    && chmod -R g+rwX ${HOME}

ENV GOPATH=/go

RUN mkdir /tmp/build && cd /tmp/build \
    && go mod init temp \
    && go get -u github.com/google/addlicense \
    && GOFLAGS="" go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 \
    && curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash \
    && chmod -R 777 /go

USER 1001

ENV PATH=$PATH:${HOME}/bin:${HOME}/.local/bin:/go/bin

RUN kubectl version --client && \
    oc version --client
