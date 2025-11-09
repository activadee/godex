package codexexec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectTargetSupportsKnownCombinations(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		expect string
	}{
		{"linux", "amd64", "x86_64-unknown-linux-musl"},
		{"linux", "arm64", "aarch64-unknown-linux-musl"},
		{"darwin", "amd64", "x86_64-apple-darwin"},
		{"darwin", "arm64", "aarch64-apple-darwin"},
		{"windows", "amd64", "x86_64-pc-windows-msvc"},
		{"windows", "arm64", "aarch64-pc-windows-msvc"},
	}

	for _, tc := range cases {
		t.Run(tc.goos+"-"+tc.goarch, func(t *testing.T) {
			info, ok := detectTarget(tc.goos, tc.goarch)
			if !ok {
				t.Fatalf("detectTarget returned false for %s/%s", tc.goos, tc.goarch)
			}
			if info.triple != tc.expect {
				t.Fatalf("expected triple %s, got %s", tc.expect, info.triple)
			}
			if !strings.Contains(info.assetName, tc.expect) {
				t.Fatalf("asset %s does not contain triple %s", info.assetName, tc.expect)
			}
		})
	}
}

func TestEnsureBundledBinaryDownloadsWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{cacheDir: tmp}

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	var called bool
	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		called = true
		if err := os.WriteFile(destPath, []byte("binary"), 0o700); err != nil {
			return err
		}
		return nil
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	path, err := ensureBundledBinary(cfg)
	if err != nil {
		t.Fatalf("ensureBundledBinary returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected downloader to be invoked")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected binary to exist: %v", err)
	}
}

func TestEnsureBundledBinarySkipsDownloadWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{cacheDir: tmp}

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	info, _ := detectTarget(runtimeGOOS, runtimeGOARCH)
	release := cfg.releaseTagName()
	targetDir := filepath.Join(tmp, release, info.triple)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destPath := filepath.Join(targetDir, info.exeName)
	if err := os.WriteFile(destPath, []byte("cached"), 0o700); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		t.Fatalf("downloader should not be called when binary exists")
		return nil
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	path, err := ensureBundledBinary(cfg)
	if err != nil {
		t.Fatalf("ensureBundledBinary returned error: %v", err)
	}
	if path != destPath {
		t.Fatalf("expected %s, got %s", destPath, path)
	}
}

func TestBundleCacheDirPrefersOptionOverEnv(t *testing.T) {
	envDir := filepath.Join(t.TempDir(), "env-cache")
	t.Setenv("GODEX_CLI_CACHE", envDir)
	explicit := filepath.Join(t.TempDir(), "explicit-cache")
	cfg := bundleConfig{cacheDir: explicit}

	got, err := cfg.cacheDirPath()
	if err != nil {
		t.Fatalf("cacheDirPath returned error: %v", err)
	}
	if got != explicit {
		t.Fatalf("cacheDirPath=%s, want %s", got, explicit)
	}
}

func TestEnsureBundledBinaryUsesProvidedReleaseTag(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{cacheDir: tmp, releaseTag: "custom-release"}
	t.Setenv("GODEX_CLI_RELEASE_TAG", "env-release")

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	var releaseUsed string
	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		releaseUsed = release
		return os.WriteFile(destPath, []byte("binary"), 0o700)
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	if _, err := ensureBundledBinary(cfg); err != nil {
		t.Fatalf("ensureBundledBinary returned error: %v", err)
	}
	if releaseUsed != "custom-release" {
		t.Fatalf("expected release custom-release, got %s", releaseUsed)
	}
}

func TestEnsureBundledBinaryVerifiesChecksums(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{
		cacheDir:    tmp,
		checksumHex: sha256Hex([]byte("binary")),
	}

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		return os.WriteFile(destPath, []byte("binary"), 0o700)
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	if _, err := ensureBundledBinary(cfg); err != nil {
		t.Fatalf("ensureBundledBinary returned error: %v", err)
	}
}

func TestEnsureBundledBinaryFailsOnChecksumMismatch(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{
		cacheDir:    tmp,
		checksumHex: strings.Repeat("00", 32),
	}

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		return os.WriteFile(destPath, []byte("binary"), 0o700)
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	if _, err := ensureBundledBinary(cfg); err == nil || !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func TestEnsureBundledBinaryRedownloadsWhenCachedChecksumMismatch(t *testing.T) {
	tmp := t.TempDir()
	cfg := bundleConfig{
		cacheDir:    tmp,
		checksumHex: sha256Hex([]byte("new")),
	}

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = "linux", "amd64"
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	info, _ := detectTarget(runtimeGOOS, runtimeGOARCH)
	release := cfg.releaseTagName()
	targetDir := filepath.Join(tmp, release, info.triple)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destPath := filepath.Join(targetDir, info.exeName)
	if err := os.WriteFile(destPath, []byte("old"), 0o700); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	var downloads int
	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		downloads++
		return os.WriteFile(destPath, []byte("new"), 0o700)
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	path, err := ensureBundledBinary(cfg)
	if err != nil {
		t.Fatalf("ensureBundledBinary returned error: %v", err)
	}
	if downloads != 1 {
		t.Fatalf("expected 1 re-download, got %d", downloads)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected binary to be rewritten with new contents")
	}
}

func TestFindCodexPathFallsBackToSystemBinary(t *testing.T) {
	tmpCache := t.TempDir()
	t.Setenv("GODEX_CLI_CACHE", tmpCache)

	originalGOOS, originalGOARCH := runtimeGOOS, runtimeGOARCH
	runtimeGOOS, runtimeGOARCH = runtime.GOOS, runtime.GOARCH
	t.Cleanup(func() {
		runtimeGOOS, runtimeGOARCH = originalGOOS, originalGOARCH
	})

	originalDownloader := downloadBinaryFunc
	downloadBinaryFunc = func(info targetInfo, release, destPath string) error {
		return fmt.Errorf("simulated download failure")
	}
	t.Cleanup(func() { downloadBinaryFunc = originalDownloader })

	tempBinDir := t.TempDir()
	dummyCodex := filepath.Join(tempBinDir, "codex")
	if runtime.GOOS == "windows" {
		dummyCodex += ".exe"
	}
	if err := os.WriteFile(dummyCodex, []byte("dummy"), 0o700); err != nil {
		t.Fatalf("write dummy binary: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

	path, err := findCodexPath(bundleConfig{})
	if err != nil {
		t.Fatalf("findCodexPath returned error: %v", err)
	}
	if !strings.HasPrefix(path, tempBinDir) {
		t.Fatalf("expected fallback path within %s, got %s", tempBinDir, path)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
