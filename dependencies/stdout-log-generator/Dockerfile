# Build the app binary
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go.mod and go.sum for dependency resolution
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY main.go ./

# Clean up unused dependencies and build
RUN go mod tidy && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o stdout-log-generator

# Use the scratch image for a minimal image
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"
WORKDIR /
COPY --from=builder /app/stdout-log-generator .
ENTRYPOINT ["/stdout-log-generator"]
