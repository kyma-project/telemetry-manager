# Build the app binary
FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o chown

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"
WORKDIR /
COPY --from=builder /app/chown .
ENTRYPOINT ["/chown"]
