# ===== Stage 1: Build frontend =====
FROM node:22-slim AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# ===== Stage 2: Build Go server (web + VPN) =====
FROM golang:1.22-alpine AS go-builder
WORKDIR /app

# Cache deps
COPY src-vpn/go.mod ./
RUN go mod download || true

# Copy source
COPY src-vpn/ ./

# Build web server (serves frontend + VPN API)
RUN CGO_ENABLED=0 go build -o /messenger-server ./cmd/webserver/

# ===== Stage 3: Runtime =====
FROM alpine:latest
RUN apk add --no-cache ca-certificates wireguard-tools iproute2 iptables tzdata sqlite

# Copy frontend
COPY --from=frontend /app/dist /app/dist-svelte

# Copy Go server
COPY --from=go-builder /messenger-server /app/messenger-server

# Ports: 9999 = web UI, 51820 = WireGuard
EXPOSE 9999 51820/udp

ENV PORT=9999
ENV DIST_DIR=/app/dist-svelte

WORKDIR /app
CMD ["/app/messenger-server"]
