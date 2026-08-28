# FILE: services/build-worker/Dockerfile
#
# This service runs Docker-in-Docker style operations.
# In production it uses a Docker socket mounted as a volume:
# docker run -v /var/run/docker.sock:/var/run/docker.sock build-worker
# The image itself does NOT bundle Docker — the socket is mounted at runtime.

FROM golang:1.26-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/trade-eval/trade-eval-platform"
LABEL org.opencontainers.image.description="Consumes build jobs from Kafka, compiles contestant images, and pushes results to MinIO"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/build-worker .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/build-worker /app/build-worker

EXPOSE 8081

USER nonroot:nonroot

ENTRYPOINT ["/app/build-worker"]
