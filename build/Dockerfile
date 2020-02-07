FROM golang:1.13.7-alpine3.11 as builder

COPY . /che-workspace-operator/
WORKDIR /che-workspace-operator/

RUN CGO_ENABLED=0 GOOS=linux go build \
  -o _output/bin/che-workspace-controller \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  cmd/manager/main.go

FROM registry.access.redhat.com/ubi8-minimal:8.1-279

# copy compilied opertator
COPY --from=builder /che-workspace-operator/_output/bin/che-workspace-controller /usr/local/bin/che-workspace-controller

ENV USER_UID=1001 \
    USER_NAME=che-workspace-controller

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/che-workspace-controller
