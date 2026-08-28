# FILE: services/telemetry-ingester/Dockerfile
FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="Consumes bot telemetry from Kafka and writes validated metrics to TimescaleDB and Redis"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/telemetry-ingester .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/telemetry-ingester /app/telemetry-ingester

EXPOSE 8083

USER nonroot:nonroot

ENTRYPOINT ["/app/telemetry-ingester"]
