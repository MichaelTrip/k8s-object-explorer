# Build stage
FROM golang:1.21-alpine AS builder

# Build arguments for version information
ARG APP_VERSION=v1.0.0
ARG BUILD_DATE=""
ARG GIT_COMMIT=""

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${APP_VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
    -o k8s-object-explorer \
    cmd/main.go

# Final stage
FROM alpine:latest

# Set environment variables
ENV PORT=8080
ENV DEBUG=false

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/k8s-object-explorer .

# Copy web assets
COPY --from=builder /app/web ./web

# Change ownership
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose port
EXPOSE ${PORT}

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:${PORT}/api/debug || exit 1

# Run the application
CMD ["./k8s-object-explorer"]
