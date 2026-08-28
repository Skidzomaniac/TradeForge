package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerfileFor_AllLanguages(t *testing.T) {
	for _, lang := range []string{"cpp", "rust", "go", "python"} {
		df := dockerfileFor(lang)
		if df == "" {
			t.Fatalf("empty Dockerfile for %s", lang)
		}
		if !strings.Contains(df, "EXPOSE 8080") {
			t.Errorf("%s Dockerfile missing EXPOSE 8080", lang)
		}
		if !strings.Contains(df, "HEALTHCHECK") {
			t.Errorf("%s Dockerfile missing HEALTHCHECK", lang)
		}
	}
	if dockerfileFor("java") != "" {
		t.Error("expected empty Dockerfile for unsupported language")
	}
}

func TestUnzip_RejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../../etc/passwd")
	_, _ = w.Write([]byte("malicious"))
	_ = zw.Close()
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	_ = os.MkdirAll(dest, 0o755)
	if err := unzip(zipPath, dest); err == nil {
		t.Fatal("expected zip-slip to be rejected")
	}
}

func TestUnzip_ExtractsValid(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "ok.zip")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("main.cpp")
	_, _ = w.Write([]byte("int main(){}"))
	_ = zw.Close()
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out")
	_ = os.MkdirAll(dest, 0o755)
	if err := unzip(zipPath, dest); err != nil {
		t.Fatalf("unzip failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "main.cpp")); err != nil {
		t.Fatalf("expected main.cpp extracted: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 3); got != "hel" {
		t.Errorf("got %q", got)
	}
	if got := truncate("hi", 5); got != "hi" {
		t.Errorf("got %q", got)
	}
}

func hasOpt(opts []string, prefix string) bool {
	for _, o := range opts {
		if strings.HasPrefix(o, prefix) {
			return true
		}
	}
	return false
}

func TestBuildSecurityOpt_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "seccomp.json")
	if err := os.WriteFile(good, []byte(`{"defaultAction":"SCMP_ACT_ERRNO"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("configured but unreadable seccomp aborts", func(t *testing.T) {
		_, err := buildSecurityOpt(Config{SeccompProfile: filepath.Join(dir, "missing.json")})
		if err == nil {
			t.Fatal("expected error for unreadable seccomp profile")
		}
	})

	t.Run("empty seccomp aborts", func(t *testing.T) {
		if _, err := buildSecurityOpt(Config{SeccompProfile: empty}); err == nil {
			t.Fatal("expected error for empty seccomp profile")
		}
	})

	t.Run("strict requires seccomp", func(t *testing.T) {
		if _, err := buildSecurityOpt(Config{StrictSandbox: true}); err == nil {
			t.Fatal("expected strict mode to require a seccomp profile")
		}
	})

	t.Run("strict requires apparmor when seccomp present", func(t *testing.T) {
		if _, err := buildSecurityOpt(Config{StrictSandbox: true, SeccompProfile: good}); err == nil {
			t.Fatal("expected strict mode to require an apparmor profile")
		}
	})

	t.Run("valid profiles produce opts", func(t *testing.T) {
		opts, err := buildSecurityOpt(Config{StrictSandbox: true, SeccompProfile: good, AppArmorProfile: "trade-eval-contestant"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasOpt(opts, "no-new-privileges") {
			t.Error("missing no-new-privileges")
		}
		if !hasOpt(opts, "seccomp=") {
			t.Error("missing seccomp opt")
		}
		if !hasOpt(opts, "apparmor=trade-eval-contestant") {
			t.Error("missing apparmor opt")
		}
	})

	t.Run("non-strict with no profiles still hardens with no-new-privileges", func(t *testing.T) {
		opts, err := buildSecurityOpt(Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasOpt(opts, "no-new-privileges") {
			t.Error("missing no-new-privileges")
		}
		if hasOpt(opts, "seccomp=") || hasOpt(opts, "apparmor=") {
			t.Error("did not expect profiles when none configured in non-strict mode")
		}
	})
}
