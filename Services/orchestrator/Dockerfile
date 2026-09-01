# FILE: services/orchestrator/Dockerfile
FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="Orchestrator for test lifecycle and crash recovery"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/orchestrator .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/orchestrator /app/orchestrator

EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/app/orchestrator"]
