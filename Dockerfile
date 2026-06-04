# Stage 1: Build the Go binary
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies for Go (lmdb-dev is required for CGO)
RUN sed -i 's/https/http/g' /etc/apk/repositories && \
    apk add --no-cache build-base lmdb-dev git ca-certificates

# Set Go Proxy and disable SumDB for maximum resilience
ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# Copy mod files and LOCAL packages first
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg

# Now download dependencies with retries and no cache
RUN go mod download -x -modcacherw

# Copy the rest of the source code
COPY . .

# ARG variables populated by buildx
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# Build the Go binary
RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-s -w" -o main .

# Stage 2: Final lightweight image
FROM alpine:3.19

WORKDIR /app

# Use a more stable mirror and install dependencies
# Note: FreeRADIUS is REMOVED. LMDB/SQLite are kept for the Go drivers.
RUN sed -i 's/https/http/g' /etc/apk/repositories && \
    apk add --no-cache \
    lmdb \
    sqlite \
    supervisor \
    procps \
    curl \
    ca-certificates

# Pre-install cloudflared
RUN ARCH=$(uname -m) && \
    case $ARCH in \
    x86_64)  CLOUDFLARED_ARCH="amd64" ;; \
    aarch64) CLOUDFLARED_ARCH="arm64" ;; \
    armv7l)  CLOUDFLARED_ARCH="arm" ;; \
    *)       CLOUDFLARED_ARCH="amd64" ;; \
    esac && \
    curl -sSL "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CLOUDFLARED_ARCH}" -o /usr/local/bin/cloudflared && \
    chmod +x /usr/local/bin/cloudflared

# Ensure data directories exist
RUN mkdir -p /app/data /var/run/supervisord /var/log/supervisord /var/log/supervisor /etc/supervisor/conf.d \
    && chmod 777 /app/data

# Copy binaries, entrypoint script, supervisord config, and all static assets from builder stage
COPY --from=builder /app/main ./
COPY --from=builder /app/docker-entrypoint.sh ./
COPY --from=builder /app/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY --from=builder /app/public ./public
COPY --from=builder /app/public_radius ./public_radius
COPY --from=builder /app/data/routing_data.json /app/data/routing_data.json
RUN sed -i 's/\r$//' docker-entrypoint.sh && chmod +x docker-entrypoint.sh && \
    sed -i 's/\r$//' /etc/supervisor/conf.d/supervisord.conf

# Expose ports
# 80: Dashboard
# 1812/udp: RADIUS Auth
# 1813/udp: RADIUS Acct
EXPOSE 80 1812/udp 1813/udp

ENV PORT=80
ENV GODEBUG=x509negativeserial=1

ENTRYPOINT ["./docker-entrypoint.sh"]
