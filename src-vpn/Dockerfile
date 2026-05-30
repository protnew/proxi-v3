FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -o /messenger-server ./cmd/webserver/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /messenger-server /app/messenger-server
COPY --from=builder /build/../dist /app/dist

EXPOSE 9999

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -q --spider http://localhost:9999/api/status || exit 1

CMD ["/app/messenger-server"]
