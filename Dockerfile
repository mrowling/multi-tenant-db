# syntax=docker/dockerfile:1
FROM golang:1.24.6-alpine AS builder

# Install build dependencies (cache this layer)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better dependency caching
COPY go.mod go.sum ./

# Download dependencies (this layer will be cached unless go.mod/go.sum changes)
ENV CGO_ENABLED=1
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN go build -ldflags="-s -w" -o /app/multitenant-db ./cmd/multi-tenant-db

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache sqlite-dev ca-certificates

# Create non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/multitenant-db /app/multitenant-db

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

EXPOSE 3306 8080

ENTRYPOINT ["/app/multitenant-db"]
