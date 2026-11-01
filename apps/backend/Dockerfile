# Stage 1: Build the Go binary
FROM golang:1.27-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates

# Copy dependency manifests to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Compile static Linux binary without CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/server ./cmd/server

# Stage 2: Minimal hardened runtime image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Create non-root unprivileged service user
RUN adduser -D -u 10001 -g "" appuser

# Copy compiled binary from builder
COPY --from=builder /out/server /app/server

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to unprivileged user
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["/app/server"]
