ARG ALPINE_VERSION

FROM --platform=$BUILDPLATFORM node:alpine${ALPINE_VERSION} AS builder

ARG TARGETOS
ARG TARGETARCH
ARG PROMETHEUS_VERSION

RUN apk add --no-cache git make musl-dev go curl bash
RUN apk --no-cache upgrade

# Configure Go
ENV GOROOT=/usr/lib/go
ENV GOPATH=/go
ENV PATH=/go/bin:$PATH


RUN mkdir -p ${GOPATH}/src ${GOPATH}/bin
WORKDIR /
RUN curl -LO https://github.com/prometheus/prometheus/archive/refs/tags/v${PROMETHEUS_VERSION}.tar.gz
RUN tar xvzf v${PROMETHEUS_VERSION}.tar.gz
RUN mv /prometheus-${PROMETHEUS_VERSION} /prometheus

WORKDIR /prometheus
COPY plugins.yml plugins.yml
COPY .promu.yml .promu.yml

# the crossbuild functionality requires to run docker in docker
# that's why we use the plain "make build" command. Only the plain build should use the targetarch, not the utilities, that's why the make call is divided into its parts, see
# https://github.com/prometheus/prometheus/blob/f1db64f4c513f9b0abaed6cac96646fadfe485ca/Makefile#L163C8-L163C51
RUN make assets npm_licenses assets-compress plugins
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} make common-build

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

COPY --from=builder /prometheus/prometheus /bin/prometheus

USER       nobody
EXPOSE     9090

ENTRYPOINT [ "/bin/prometheus" ]
