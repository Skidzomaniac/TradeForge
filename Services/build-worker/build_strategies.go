package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// dockerfileFor returns the language-specific Dockerfile contents.
func dockerfileFor(language string) string {
	switch language {
	case "cpp":
		return `FROM alpine:3.19 AS builder
RUN apk add --no-cache g++ make
WORKDIR /src
COPY src/ .
RUN timeout 120 g++ -O2 -std=c++17 -pthread -o /orderbook main.cpp 2>&1

FROM alpine:3.19
RUN apk add --no-cache libstdc++ && adduser -D -u 1001 contestant
WORKDIR /app
COPY --from=builder /orderbook /app/orderbook
USER contestant
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=3s --retries=15 CMD wget -q -O- http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/orderbook"]
`
	case "rust":
		return `FROM rust:1.77-alpine AS builder
RUN apk add --no-cache musl-dev
WORKDIR /src
COPY src/ .
RUN timeout 180 cargo build --release 2>&1
RUN cp target/release/$(ls target/release | grep -v '\.' | head -1) /orderbook

FROM alpine:3.19
RUN adduser -D -u 1001 contestant
WORKDIR /app
COPY --from=builder /orderbook /app/orderbook
USER contestant
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=3s --retries=15 CMD wget -q -O- http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/orderbook"]
`
	case "go":
		return `FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY src/ .
RUN timeout 180 go build -o /orderbook ./... 2>&1

FROM alpine:3.19
RUN adduser -D -u 1001 contestant
WORKDIR /app
COPY --from=builder /orderbook /app/orderbook
USER contestant
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=3s --retries=15 CMD wget -q -O- http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/orderbook"]
`
	case "python":
		return `FROM python:3.12-alpine
RUN adduser -D -u 1001 contestant
WORKDIR /app
COPY src/ .
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
USER contestant
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=3s --retries=15 CMD wget -q -O- http://localhost:8080/health || exit 1
ENTRYPOINT ["python", "main.py"]
`
	default:
		return ""
	}
}

// buildContainer builds a Docker image from contestant source.
func buildContainer(ctx context.Context, cli *client.Client, submissionID, language, srcDir string) (imageName string, buildLogs string, err error) {
	df := dockerfileFor(language)
	if df == "" {
		return "", "", fmt.Errorf("unsupported language %q", language)
	}

	tarball, err := tarBuildContext(srcDir, df)
	if err != nil {
		return "", "", fmt.Errorf("tar build context: %w", err)
	}

	imageName = "trade-eval-contestant:" + submissionID
	resp, err := cli.ImageBuild(ctx, tarball, types.ImageBuildOptions{
		Tags:        []string{imageName},
		Dockerfile:  "Dockerfile",
		Remove:      true,
		ForceRemove: true,
		NoCache:     false,
	})
	if err != nil {
		return "", "", fmt.Errorf("image build: %w", err)
	}
	defer resp.Body.Close()

	var logBuf bytes.Buffer
	if _, err := io.Copy(&logBuf, io.LimitReader(resp.Body, 1<<20)); err != nil {
		return "", logBuf.String(), fmt.Errorf("read build logs: %w", err)
	}
	logs := logBuf.String()
	if strings.Contains(logs, `"errorDetail"`) || strings.Contains(logs, `"error":`) {
		return "", logs, fmt.Errorf("build failed; see logs")
	}
	return imageName, logs, nil
}

// tarBuildContext walks srcDir into a tar stream and injects the Dockerfile.
func tarBuildContext(srcDir, dockerfile string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Inject the generated Dockerfile at the root.
	if err := writeTarFile(tw, "Dockerfile", []byte(dockerfile)); err != nil {
		return nil, err
	}

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeTarFile(tw, filepath.ToSlash(filepath.Join("src", rel)), data)
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
