# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

# Install required packages
RUN apk add --no-cache git build-base ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version information
ARG VERSION=0.1.0
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s \
    -X github.com/R3E-Network/service_layer/pkg/version.Version=${VERSION} \
    -X github.com/R3E-Network/service_layer/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/R3E-Network/service_layer/pkg/version.BuildTime=${BUILD_TIME}" \
    -o appserver ./cmd/appserver

RUN CGO_ENABLED=0 GOOS=linux go build -o slctl ./cmd/slctl
RUN CGO_ENABLED=0 GOOS=linux go build -o neo-indexer ./cmd/neo-indexer

# Stage 2: Minimal runtime image
FROM alpine:3.19

# Install CA certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 app && \
    adduser -u 1001 -G app -s /bin/sh -D app

# Create app directories
RUN mkdir -p /app/config /app/data /app/logs && \
    chown -R app:app /app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/appserver .
COPY --from=builder /app/slctl .
COPY --from=builder /app/neo-indexer .

# Copy configuration files
COPY --from=builder /app/configs/config.yaml ./config/

# Set environment variables
ENV CONFIG_FILE=/app/config/config.yaml
ENV DATABASE_URL=postgres://postgres:postgres@localhost:5432/service_layer?sslmode=disable
ENV LOG_LEVEL=info
ENV LOG_FORMAT=json
ENV PORT=8080

# Expose the port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Use non-root user
USER app

# Set the entrypoint
ENTRYPOINT ["/app/appserver"]
