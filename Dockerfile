# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.24.4-alpine3.21 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_COMMIT_SHA

WORKDIR /telemetry-manager-workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./
# Copy the go source
RUN go mod download
# Added for test purposes

COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY internal/ internal/
COPY webhook/ webhook/

COPY .git .git

RUN apk add --no-cache git
RUN git config --global --add safe.directory /telemetry-manager-workspace && git describe --tags

# Clean up unused (test) dependencies and build
RUN export TAG=$(git describe --tags) && \
    export COMMIT=${BUILD_COMMIT_SHA} && \
    export TREESTATE=$(git diff -s --exit-code && echo "clean" || echo "modified") && \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-X github.com/kyma-project/telemetry-manager/internal/build.gitCommit=${COMMIT} \
    -X github.com/kyma-project/telemetry-manager/internal/build.gitTag=${TAG} \
    -X github.com/kyma-project/telemetry-manager/internal/build.gitTreeState=${TREESTATE}" \
    -a -o manager main.go

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

WORKDIR /

COPY --from=builder /telemetry-manager-workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
