# Dockerfile for SASMAN Central Gateway Server
# Build from the project ROOT directory:
#   docker build -f server/Dockerfile -t sasman-central .

FROM golang:alpine AS builder

WORKDIR /app

RUN sed -i 's/https/http/g' /etc/apk/repositories && \
    apk add --no-cache build-base git ca-certificates lmdb-dev

ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# Copy mod files and LOCAL packages first
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg

# Download dependencies
RUN go mod download -x -modcacherw

# Copy entire source (server/ imports pkg/ from root)
COPY . .

# Build both the central server and the cloud agent binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o central-server ./server
RUN CGO_ENABLED=1 CGO_CFLAGS="-DMDB_USE_ROBUST=0" go build -ldflags="-s -w" -o sasman-agent ./agent

# ─── Final lightweight image ───────────────────────────────────────────────────
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite docker-cli tar curl lmdb && \
    mkdir -p /app/data /app/data/releases /app/data/tenants

COPY --from=builder /app/central-server ./central-server
COPY --from=builder /app/sasman-agent ./sasman-agent
COPY --from=builder /app/agent/web_radius ./web_radius
COPY --from=builder /app/agent/web_hotspot ./web_hotspot
COPY --from=builder /app/agent/web_dashboard ./web_dashboard

EXPOSE 8080 2083

ENV ADDR=:8080
ENV SASMAN_CENTRAL_DOMAIN=sas-man.net
ENV SASMAN_DB_PATH=/app/data/sasman-central.db

ENTRYPOINT ["./central-server"]
