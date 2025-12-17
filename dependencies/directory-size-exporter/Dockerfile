FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/

COPY main.go go.* /src/
COPY internal/ /src/internal/

# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN go mod tidy && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /bin/exporter

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

COPY --from=build /bin/exporter /bin/exporter

USER 65532:65532

ENTRYPOINT ["/bin/exporter"]
