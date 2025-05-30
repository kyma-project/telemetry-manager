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

RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} make build

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"

COPY --from=builder /prometheus/prometheus /bin/prometheus

USER       nobody
EXPOSE     9090

ENTRYPOINT [ "/bin/prometheus" ]
