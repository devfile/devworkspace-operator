FROM golang:1.13.7-alpine3.11 as builder

WORKDIR /che-workspace-operator

# Populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .
RUN go mod download

# copy the rest of the sources code
COPY . .
# compile workspace controller binaries
RUN CGO_ENABLED=0 GOOS=linux go build \
  -o _output/bin/che-workspace-controller \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  cmd/manager/main.go

FROM registry.access.redhat.com/ubi8-minimal:8.1-279
COPY --from=builder /che-workspace-operator/_output/bin/che-workspace-controller /usr/local/bin/che-workspace-controller
COPY --from=builder /che-workspace-operator/internal-registry  internal-registry

ENV USER_UID=1001 \
    USER_NAME=che-workspace-controller

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/che-workspace-controller
