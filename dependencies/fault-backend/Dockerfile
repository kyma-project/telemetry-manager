FROM --platform=$BUILDPLATFORM golang:1.26.2-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o fault-backend

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"
WORKDIR /
COPY --from=builder /app/fault-backend .
ENTRYPOINT ["/fault-backend"]
