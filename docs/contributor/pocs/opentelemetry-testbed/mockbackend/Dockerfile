# Start from the official Golang image for building
FROM golang:1.24.4-alpine3.21 AS builder

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o mockbackend .

# Use a minimal base image for running
FROM alpine:3.19

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/mockbackend .

# Run the application
ENTRYPOINT ["./mockbackend"]
