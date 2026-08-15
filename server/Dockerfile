# Dockerfile for SASMAN Central Gateway Server
# Build from the project ROOT directory:
#   docker build -f server/Dockerfile -t sasman-central .

FROM golang:alpine AS builder

WORKDIR /app

RUN sed -i 's/https/http/g' /etc/apk/repositories && \
    apk add --no-cache build-base git ca-certificates

ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct
ENV GOSUMDB=off

# Copy mod files and LOCAL packages first
COPY go.mod go.sum ./
COPY routeros_pkg ./routeros_pkg

# Download dependencies
RUN go mod download -x -modcacherw

# Copy entire source (server/ imports pkg/ from root)
COPY . .

# Build ONLY the central server binary (CGO disabled for smaller image)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o central-server ./server

# ─── Final lightweight image ───────────────────────────────────────────────────
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite docker-cli tar curl && \
    mkdir -p /app/data /app/data/releases

COPY --from=builder /app/central-server ./central-server

EXPOSE 8080

ENV ADDR=:8080
ENV SASMAN_CENTRAL_DOMAIN=sas-man.net
ENV SASMAN_DB_PATH=/app/data/sasman-central.db

ENTRYPOINT ["./central-server"]
