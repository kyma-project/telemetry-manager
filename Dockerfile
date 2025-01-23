# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.23.5-alpine3.21 AS builder

WORKDIR /telemetry-manager-workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY internal/ internal/
COPY webhook/ webhook/

# Clean up unused (test) dependencies and build
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go mod tidy && go build -a -o manager main.go

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

WORKDIR /

COPY --from=builder /telemetry-manager-workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
