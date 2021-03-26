# Build the project-clone binary
FROM quay.io/libpod/golang:1.13 as builder

ENV GOPATH=/go/
USER root

WORKDIR /project-clone
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# compile project-clone binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
  -a -o _output/bin/project-clone \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  project-clone/main.go

FROM registry.access.redhat.com/ubi8-minimal:8.2-349
WORKDIR /
COPY --from=builder /project-clone/_output/bin/project-clone /usr/local/bin/project-clone

ENV USER_UID=1001 \
    USER_NAME=project-clone

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/project-clone
