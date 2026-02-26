# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-w -s" -o /arda-notification ./cmd/server

# ─────────────────────────────────────────────────────────────

# Runtime stage — minimal image
FROM scratch

COPY --from=builder /arda-notification /arda-notification
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8090

ENTRYPOINT ["/arda-notification"]
