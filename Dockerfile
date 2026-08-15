# Stage 1: Build the Go binary with CGO & LMDB
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies for CGO (lmdb-dev and gcc/build-base)
RUN apk add --no-cache build-base lmdb-dev git ca-certificates

# Set Go Proxy and disable SumDB for speed and resilience
ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# Copy mod files and LOCAL packages first
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg

# Download dependencies
RUN go mod download -x -modcacherw

# Copy source code
COPY . .

# Set CGO flags to disable robust mutexes for full MikroTik RouterOS ARM64/ARMv7 kernel compatibility
ENV CGO_CFLAGS="-DMDB_USE_ROBUST=0"

# Build with CGO enabled
RUN CGO_ENABLED=1 CGO_CFLAGS="-DMDB_USE_ROBUST=0" go build -ldflags="-s -w" -o main .

# Stage 2: Final lightweight runtime image
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache \
    lmdb \
    sqlite \
    supervisor \
    procps \
    curl \
    ca-certificates

# Ensure data directories exist
RUN mkdir -p /app/data /var/run/supervisord /var/log/supervisord /var/log/supervisor /etc/supervisor/conf.d \
    && chmod 777 /app/data

# Copy binaries and configs from builder stage
COPY --from=builder /app/main ./
COPY --from=builder /app/docker-entrypoint.sh ./
COPY --from=builder /app/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY --from=builder /app/public ./public
COPY --from=builder /app/public_radius ./public_radius
COPY --from=builder /app/data/routing_data.json /app/data/routing_data.json

EXPOSE 88 1812/udp 1813/udp

ENV SASMAN_DATA_DIR=/app/data
ENV SASMAN_PORT=88

ENTRYPOINT ["/app/docker-entrypoint.sh"]
