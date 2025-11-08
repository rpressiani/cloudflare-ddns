# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies (if any)
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary
# CGO_ENABLED=0 for a static binary
# -ldflags for smaller binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o cf-ddns \
    main.go

# Final stage - distroless image (even more minimal and secure than alpine)
FROM gcr.io/distroless/static:nonroot

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/cf-ddns .

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Distroless runs as nonroot user (UID 65532) by default
# No need to create user or change ownership

# Run the application
CMD ["./cf-ddns"]
