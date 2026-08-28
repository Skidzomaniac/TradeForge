// Package security provides input validation and upload safety checks.
package security

import (
	"path/filepath"
	"strings"

	"github.com/trade-eval/submission-api/apierrors"
)

var allowedLanguages = map[string]bool{"cpp": true, "rust": true, "go": true, "python": true}
var allowedPersonas = map[string]bool{"market_maker": true, "aggressive_taker": true, "spammer": true, "whale": true}

// ValidateLanguage checks the language is supported.
func ValidateLanguage(language string) error {
	if !allowedLanguages[strings.ToLower(language)] {
		return &apierrors.ErrValidation{Field: "language", Message: "language must be one of cpp, rust, go, python"}
	}
	return nil
}

// ValidateFilename rejects path traversal and suspicious names.
func ValidateFilename(name string) error {
	base := filepath.Base(name)
	if base != name || strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.ContainsRune(name, 0) {
		return &apierrors.ErrValidation{Field: "file", Message: "INVALID_FILENAME"}
	}
	return nil
}

// ValidateFileSize enforces a sane min/max upload size.
func ValidateFileSize(size, maxBytes int64) error {
	if size < 1024 {
		return &apierrors.ErrValidation{Field: "file", Message: "file too small"}
	}
	if size > maxBytes {
		return &apierrors.ErrValidation{Field: "file", Message: "file too large"}
	}
	return nil
}

// ValidateTestParams clamps/validates test-creation parameters.
func ValidateTestParams(durationSeconds, botCount int, personas []string) error {
	if durationSeconds != 0 && (durationSeconds < 30 || durationSeconds > 600) {
		return &apierrors.ErrValidation{Field: "duration_seconds", Message: "must be between 30 and 600"}
	}
	if botCount != 0 && (botCount < 1 || botCount > 1000) {
		return &apierrors.ErrValidation{Field: "bot_count", Message: "must be between 1 and 1000"}
	}
	for _, p := range personas {
		if !allowedPersonas[p] {
			return &apierrors.ErrValidation{Field: "bot_personas", Message: "unknown persona: " + p}
		}
	}
	return nil
}
