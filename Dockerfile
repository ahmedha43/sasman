# Stage 1: Build the Go binary natively on host platform
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

WORKDIR /app

# Install git and certs
RUN apk add --no-cache git ca-certificates

# Set Go Proxy and disable SumDB for speed and resilience
ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# Copy mod files and LOCAL packages first
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg

# Download dependencies
RUN go mod download -x -modcacherw

# Copy the rest of the source code
COPY . .

# ARG variables populated by buildx
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# Build the Go binary with instant native cross-compilation
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-s -w" -o main .

# Stage 2: Final lightweight image
FROM alpine:3.19

WORKDIR /app

# Note: FreeRADIUS is REMOVED. LMDB/SQLite are kept for the Go drivers.
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
