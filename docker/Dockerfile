# Service Layer Dockerfile
# Multi-stage build for optimized production image

# =============================================================================
# Stage 1: Build Go binary
# =============================================================================
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /build/server \
    ./cmd/server/

# =============================================================================
# Stage 2: Production image
# =============================================================================
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1000 service && \
    adduser -u 1000 -G service -s /bin/sh -D service

# Copy binary from builder
COPY --from=builder /build/server /app/server

# Create directories for sealed storage
RUN mkdir -p /app/sealed_store && \
    chown -R service:service /app

# Switch to non-root user
USER service

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/app/server"]
CMD ["--mode=simulation", "--http=:8080", "--metrics=:9090"]
