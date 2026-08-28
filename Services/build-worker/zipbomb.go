package main

import (
	"archive/zip"
	"errors"
	"strings"
)

// checkZipBomb inspects a zip archive for decompression-bomb / zip-slip patterns
// before any extraction occurs.
func checkZipBomb(zr *zip.Reader) error {
	const (
		maxUncompressed = int64(500) * 1024 * 1024
		maxRatio        = 100
		maxEntries      = 10000
	)
	if len(zr.File) > maxEntries {
		return errors.New("zip bomb: too many entries")
	}
	var unc, comp int64
	for _, f := range zr.File {
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") {
			return errors.New("path traversal in zip")
		}
		unc += int64(f.UncompressedSize64)
		comp += int64(f.CompressedSize64)
		if unc > maxUncompressed {
			return errors.New("zip bomb: uncompressed size exceeds limit")
		}
	}
	if comp > 0 && unc/comp > maxRatio {
		return errors.New("zip bomb: compression ratio too high")
	}
	return nil
}
