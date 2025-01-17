FROM golang:1.23 AS builder
RUN mkdir /app
ADD . /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./...

FROM scratch
LABEL source=git@github.com:kyma-project/telemetry-manager.git

COPY --from=builder /app .
EXPOSE 8080

CMD ["./main"]
