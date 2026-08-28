package security

import (
	"archive/zip"
	"errors"
	"strings"
)

// ErrZipBomb indicates a decompression-bomb or malformed archive.
var ErrZipBomb = errors.New("ZIP_BOMB")

// ErrPathTraversal indicates a zip entry escaping the extraction root.
var ErrPathTraversal = errors.New("PATH_TRAVERSAL")

const (
	maxUncompressed = int64(500) * 1024 * 1024 // 500MB
	maxRatio        = 100                       // compressed:uncompressed
	maxEntries      = 10000
)

// CheckZipReader inspects a zip without extracting it for bomb/slip patterns.
func CheckZipReader(zr *zip.Reader) error {
	if len(zr.File) > maxEntries {
		return ErrZipBomb
	}
	var totalUncompressed, totalCompressed int64
	for _, f := range zr.File {
		name := f.Name
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return ErrPathTraversal
		}
		totalUncompressed += int64(f.UncompressedSize64)
		totalCompressed += int64(f.CompressedSize64)
		if totalUncompressed > maxUncompressed {
			return ErrZipBomb
		}
	}
	if totalCompressed > 0 && totalUncompressed/totalCompressed > maxRatio {
		return ErrZipBomb
	}
	return nil
}
