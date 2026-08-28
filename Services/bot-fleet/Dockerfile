# FILE: services/bot-fleet/Dockerfile
FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="Spawns thousands of simulated trading bots and produces telemetry to Kafka"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bot-fleet .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/bot-fleet /app/bot-fleet

EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/app/bot-fleet"]
