# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o novagate-acl \
    .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/novagate-acl .
COPY --from=builder /build/config.example.yaml ./config.yaml

# Create non-root user
RUN addgroup -g 1000 acl && \
    adduser -D -u 1000 -G acl acl && \
    chown -R acl:acl /app

USER acl

EXPOSE 8888

ENTRYPOINT ["./novagate-acl"]
CMD ["-config", "/app/config.yaml"]
