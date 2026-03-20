FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o mock-backend

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/kyma-project/telemetry-manager"
WORKDIR /
COPY --from=builder /app/mock-backend .
ENTRYPOINT ["/mock-backend"]
