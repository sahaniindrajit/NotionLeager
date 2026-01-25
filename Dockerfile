# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o notionledger ./cmd/main.go

# Final stage
FROM alpine:3.19

WORKDIR /app

# Add ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/notionledger .

# Expose port (default 8080, can be overridden via env)
EXPOSE 8080

# Run the application
CMD ["./notionledger"]
