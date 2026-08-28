package main

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"testing"
)

// zipEntry builds a single-entry zip in memory and returns that entry.
func zipEntry(t *testing.T, name string, size int) *zip.File {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("a"), size)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr.File[0]
}

func TestExtractFileWithinBudget(t *testing.T) {
	f := zipEntry(t, "main.cpp", 100)
	n, err := extractFile(f, filepath.Join(t.TempDir(), "main.cpp"), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 100 {
		t.Fatalf("want 100 bytes written, got %d", n)
	}
}

// TestExtractFileExceedsBudget is the regression for item 10: extraction aborts
// with an error instead of silently truncating once the byte budget is exceeded.
func TestExtractFileExceedsBudget(t *testing.T) {
	f := zipEntry(t, "big.bin", 500)
	if _, err := extractFile(f, filepath.Join(t.TempDir(), "big.bin"), 100); err == nil {
		t.Fatal("expected an error when the entry exceeds the remaining budget")
	}
}

func TestExtractFileNoBudget(t *testing.T) {
	f := zipEntry(t, "x", 1)
	if _, err := extractFile(f, filepath.Join(t.TempDir(), "x"), 0); err == nil {
		t.Fatal("expected an error when no budget remains")
	}
}
