# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /news-server ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /news-server .

# Copy configuration
COPY configs/config.yaml ./configs/
COPY migrations ./migrations/

# Create data directory
RUN mkdir -p /data

# Expose port
EXPOSE 8080

# Run the server
CMD ["./news-server"]
