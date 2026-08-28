# FILE: services/submission-api/Dockerfile
FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="HTTP API for receiving and validating contestant order book submissions"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/submission-api .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/submission-api /app/submission-api
COPY --from=builder /build/migrations /app/migrations

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/submission-api"]
