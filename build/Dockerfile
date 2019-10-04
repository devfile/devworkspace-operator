FROM registry.access.redhat.com/ubi8/ubi
RUN curl https://copr.fedorainfracloud.org/coprs/vbatts/bazel/repo/epel-7/vbatts-bazel-epel-7.repo > /etc/yum.repos.d/bazel.repo && \
    yum install -y bazel && \
    yum clean all

ENV OPERATOR=/usr/local/bin/che-workspace-crd-operator \
    USER_UID=1001 \
    USER_NAME=che-workspace-crd-operator

# install operator binary
COPY build/_output/bin/che-workspace-crd-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
