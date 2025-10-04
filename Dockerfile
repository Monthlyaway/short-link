# Multi-stage build for Go application
FROM golang:1.23-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o url-shortener ./cmd/server

# Final stage - minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/url-shortener .

# Copy config directory
COPY --from=builder /app/config ./config

# Expose port
EXPOSE 8080

# Run the application
CMD ["./url-shortener"]
