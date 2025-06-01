# Stage 1: Build the application
FROM golang:1.19-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o servicelayer ./cmd/servicelayer

# Stage 2: Create a minimalist runtime image
FROM alpine:3.22

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create a non-root user to run the application
RUN adduser -D -g '' appuser
USER appuser

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/servicelayer .

# Create config directory
RUN mkdir -p /app/config

# Command to run
ENTRYPOINT ["/app/servicelayer", "-config", "/app/config/config.json"]