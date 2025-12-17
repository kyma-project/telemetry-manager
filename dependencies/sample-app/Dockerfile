# Build the app binary
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

# Copy the project
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY *.go .

# Clean up unused (test) dependencies and build
RUN go mod tidy && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o main ./...

# Use the scratch image for a minimal image
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

# Copy the binary, no further dependencies
COPY --from=builder /app/main .
EXPOSE 8080

CMD ["./main"]
