# Kaniko-based build image for compiling contestant code without a Docker daemon
# (rootless, suitable for Kubernetes). The build-worker shells out to executor.
FROM gcr.io/kaniko-project/executor:latest AS kaniko

FROM golang:1.26-alpine
RUN apk add --no-cache ca-certificates
COPY --from=kaniko /kaniko /kaniko
ENV PATH="/kaniko:${PATH}"
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/build-worker .
ENTRYPOINT ["/app/build-worker"]
