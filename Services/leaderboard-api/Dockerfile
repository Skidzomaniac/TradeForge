# FILE: services/leaderboard-api/Dockerfile
FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="Serves real-time leaderboard data via REST and WebSocket endpoints"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/leaderboard-api .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/leaderboard-api /app/leaderboard-api

EXPOSE 8084

USER nonroot:nonroot

ENTRYPOINT ["/app/leaderboard-api"]
