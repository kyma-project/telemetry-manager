# Start from the official Golang image for building
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o loadgenerator .

# Use a minimal base image for running
FROM alpine:latest

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/loadgenerator .

# Run the application
ENTRYPOINT ["./loadgenerator"]
