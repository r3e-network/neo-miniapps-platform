# Stage 1: Build the refactored appserver
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor ./vendor
ENV GOFLAGS=-mod=vendor

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o appserver ./cmd/appserver

# Stage 2: Minimal runtime image
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/appserver ./appserver
COPY configs/config.yaml ./config.yaml

ENV CONFIG_FILE=/app/config.yaml

ENTRYPOINT ["/app/appserver"]
