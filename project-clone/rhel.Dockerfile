# Build the manager binary
FROM registry.redhat.io/rhel8/go-toolset:1.13.4-27 as builder

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

# compile workspace controller binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
  -a -o _output/bin/project-clone \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  project-clone/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi8-minimal:8.2-349
WORKDIR /
COPY --from=builder /devworkspace-operator/_output/bin/project-clone /usr/local/bin/project-clone

USER nonroot:nonroot

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/project-clone
