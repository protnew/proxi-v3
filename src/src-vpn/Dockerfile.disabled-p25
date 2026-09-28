FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /messenger-server ./cmd/webserver/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget

WORKDIR /app
COPY --from=builder /messenger-server .
COPY ../dist ./dist/ 2>/dev/null || true

EXPOSE 8080

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --retries=3   CMD wget -q --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./messenger-server"]
