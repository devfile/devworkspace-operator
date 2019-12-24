FROM registry.access.redhat.com/ubi8/ubi

RUN dnf install -y dnf-plugins-core && \
       dnf copr -y enable vbatts/bazel && \
       dnf install -y bazel

ENV OPERATOR=/usr/local/bin/che-workspace-crd-operator \
    USER_UID=1001 \
    USER_NAME=che-workspace-crd-operator

# install operator binary
COPY build/_output/bin/che-workspace-crd-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
