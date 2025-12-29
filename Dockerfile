# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-s -w' -o s3dir ./cmd/s3dir

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 s3dir && \
    adduser -D -u 1000 -G s3dir s3dir

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/s3dir .

# Create data directory
RUN mkdir -p /data && chown -R s3dir:s3dir /data

# Switch to non-root user
USER s3dir

# Expose default port
EXPOSE 8000

# Set default data directory
ENV S3DIR_DATA_DIR=/data

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8000/ || exit 1

# Run the application
CMD ["./s3dir"]
