# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download


# Copy source code
COPY . .

# Build binary
RUN go build -o bin/epoch ./cmd/web

# Final image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache tzdata
ENV TZ=America/New_York
ENV DB_HOST=host.docker.internal

# Copy built binary and static files
COPY --from=builder /app/bin/epoch .
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

# For timezone

# Expose port
EXPOSE 8080

# Run the server
CMD ["./epoch"]
