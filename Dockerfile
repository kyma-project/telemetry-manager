# Build the manager binary
FROM golang:1.24.2-alpine3.21 AS builder

WORKDIR /telemetry-manager-workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Copy the git config needed for git describe
COPY .git .git
# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY internal/ internal/
COPY webhook/ webhook/

RUN apk add --no-cache git
# Clean up unused (test) dependencies and build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go mod tidy && go build -ldflags="-X main.version=$(git rev-parse --short HEAD)" -a -o manager main.go

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

WORKDIR /

COPY --from=builder /telemetry-manager-workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
