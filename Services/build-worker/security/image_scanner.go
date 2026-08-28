package security

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ErrScannerUnavailable is returned by Scan when Trivy is not installed. Callers
// in strict mode treat this as a launch-blocking failure; in dev they log and
// continue.
var ErrScannerUnavailable = errors.New("image scanner unavailable: trivy not found")

// ImageScanner runs Trivy against built contestant images. If Trivy is not
// installed it reports ErrScannerUnavailable so the caller decides whether to
// fall open (dev) or fail closed (strict/production).
type ImageScanner struct {
	logger    *slog.Logger
	available bool
}

// NewImageScanner constructs the scanner, detecting whether trivy is present.
func NewImageScanner(logger *slog.Logger) *ImageScanner {
	_, err := exec.LookPath("trivy")
	if err != nil {
		logger.Warn("trivy not found; image scanning disabled")
	}
	return &ImageScanner{logger: logger, available: err == nil}
}

// ScanResult reports the worst severity found.
type ScanResult struct {
	HasCritical bool
	HasHigh     bool
	Output      string
}

// Available reports whether Trivy was found on PATH at construction.
func (s *ImageScanner) Available() bool { return s.available }

// Scan runs `trivy image --severity HIGH,CRITICAL`. CRITICAL findings should
// block the container from starting. It returns ErrScannerUnavailable if Trivy
// is not installed, and a non-nil error if the scan itself fails to run, so a
// caller in strict mode can refuse to launch rather than fall open.
func (s *ImageScanner) Scan(ctx context.Context, imageName string) (ScanResult, error) {
	if !s.available {
		return ScanResult{}, ErrScannerUnavailable
	}
	cmd := exec.CommandContext(ctx, "trivy", "image",
		"--quiet", "--severity", "HIGH,CRITICAL", "--no-progress", imageName)
	out, err := cmd.CombinedOutput()
	res := ScanResult{Output: string(out)}
	// trivy exit code is 0 unless --exit-code is set; we parse the text for the
	// findings. A non-zero exit therefore signals a real execution failure
	// (image missing, bad args, daemon error), not a vulnerability finding.
	res.HasCritical = strings.Contains(res.Output, "CRITICAL")
	res.HasHigh = strings.Contains(res.Output, "HIGH")
	if err != nil {
		s.logger.Warn("trivy scan returned error", "error", err, "image", imageName)
		return res, fmt.Errorf("trivy scan failed for %q: %w", imageName, err)
	}
	return res, nil
}
