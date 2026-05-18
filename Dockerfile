# ===== Stage 1: Build frontend =====
FROM node:22-slim AS frontend
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY index.html vite.config.js ./
COPY src/ ./src/
RUN npm run build

# ===== Stage 2: Build Go server (web + VPN) =====
FROM golang:latest AS go-builder
WORKDIR /app

# Cache deps
COPY src-vpn/go.mod ./
RUN go mod download || true

# Copy source
COPY src-vpn/ ./

# Build web server (serves frontend + VPN API)
RUN CGO_ENABLED=0 go build -o /messenger-server ./cmd/webserver/

# ===== Stage 3: Runtime =====
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wireguard-tools iproute2 iptables \
    && rm -rf /var/lib/apt/lists/*

# Copy frontend
COPY --from=frontend /app/dist /app/dist

# Copy Go server
COPY --from=go-builder /messenger-server /app/messenger-server

# Ports: 8080 = web UI, 51820 = WireGuard
EXPOSE 8080 51820/udp

ENV PORT=8080
ENV DIST_DIR=/app/dist

WORKDIR /app
CMD ["/app/messenger-server"]
