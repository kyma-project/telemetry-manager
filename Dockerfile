# Build the manager binary
FROM europe-docker.pkg.dev/kyma-project/prod/external/library/golang:1.23.1-alpine3.20 AS builder

WORKDIR /telemetry-manager-workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY .git .git
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY internal/ internal/
COPY webhook/ webhook/

RUN apk add git

# Clean up unused (test) dependencies and build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go mod tidy && go build -ldflags="-X main.version=$(git rev-parse --short HEAD)" -a -o manager main.go

FROM scratch

WORKDIR /

COPY --from=builder /telemetry-manager-workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
