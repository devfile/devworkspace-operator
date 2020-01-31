FROM golang:1.13.7-alpine3.11 as builder

COPY . /che-workspace-crd-operator/
WORKDIR /che-workspace-crd-operator/

RUN CGO_ENABLED=0 GOOS=linux go build \
  -o _output/bin/che-workspace-crd-operator \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  cmd/manager/main.go

FROM registry.access.redhat.com/ubi8-minimal:8.1-279

# copy compilied opertator
COPY --from=builder /che-workspace-crd-operator/_output/bin/che-workspace-crd-operator /usr/local/bin/che-workspace-crd-operator

ENV OPERATOR=/usr/local/bin/che-workspace-crd-operator \
    USER_UID=1001 \
    USER_NAME=che-workspace-crd-operator

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
