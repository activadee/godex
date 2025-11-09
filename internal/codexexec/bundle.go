package codexexec

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type archiveKind int

const (
	archiveTarGz archiveKind = iota
	archiveZip
)

const defaultCodexReleaseTag = "rust-v0.55.0"

var ErrChecksumMismatch = errors.New("codex bundle checksum mismatch")

type bundleConfig struct {
	cacheDir    string
	releaseTag  string
	checksumHex string
}

func (cfg bundleConfig) cacheDirPath() (string, error) {
	if dir := strings.TrimSpace(cfg.cacheDir); dir != "" {
		return dir, nil
	}
	if override := strings.TrimSpace(os.Getenv("GODEX_CLI_CACHE")); override != "" {
		return override, nil
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "godex", "codex"), nil
	}
	return filepath.Join(os.TempDir(), "godex", "codex"), nil
}

func (cfg bundleConfig) releaseTagName() string {
	if tag := strings.TrimSpace(cfg.releaseTag); tag != "" {
		return tag
	}
	if env := strings.TrimSpace(os.Getenv("GODEX_CLI_RELEASE_TAG")); env != "" {
		return env
	}
	return defaultCodexReleaseTag
}

func (cfg bundleConfig) checksumValue() (string, error) {
	value := strings.TrimSpace(cfg.checksumHex)
	if value == "" {
		value = strings.TrimSpace(os.Getenv("GODEX_CLI_CHECKSUM"))
	}
	if value == "" {
		return "", nil
	}
	normalized, err := normalizeChecksum(value)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

func normalizeChecksum(value string) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(value))
	if cleaned == "" {
		return "", nil
	}
	if _, err := hex.DecodeString(cleaned); err != nil {
		return "", fmt.Errorf("invalid checksum value %q: %w", value, err)
	}
	return cleaned, nil
}

var downloadBinaryFunc = downloadBinaryFromRelease
var runtimeGOOS = runtime.GOOS
var runtimeGOARCH = runtime.GOARCH

type targetInfo struct {
	triple     string
	assetName  string
	archive    archiveKind
	binaryName string
	exeName    string
}

func detectTarget(goos, goarch string) (targetInfo, bool) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return targetInfo{
				triple:     "x86_64-unknown-linux-musl",
				assetName:  "codex-x86_64-unknown-linux-musl.tar.gz",
				archive:    archiveTarGz,
				binaryName: "codex-x86_64-unknown-linux-musl",
				exeName:    "codex",
			}, true
		case "arm64":
			return targetInfo{
				triple:     "aarch64-unknown-linux-musl",
				assetName:  "codex-aarch64-unknown-linux-musl.tar.gz",
				archive:    archiveTarGz,
				binaryName: "codex-aarch64-unknown-linux-musl",
				exeName:    "codex",
			}, true
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return targetInfo{
				triple:     "x86_64-apple-darwin",
				assetName:  "codex-x86_64-apple-darwin.tar.gz",
				archive:    archiveTarGz,
				binaryName: "codex-x86_64-apple-darwin",
				exeName:    "codex",
			}, true
		case "arm64":
			return targetInfo{
				triple:     "aarch64-apple-darwin",
				assetName:  "codex-aarch64-apple-darwin.tar.gz",
				archive:    archiveTarGz,
				binaryName: "codex-aarch64-apple-darwin",
				exeName:    "codex",
			}, true
		}
	case "windows":
		switch goarch {
		case "amd64":
			return targetInfo{
				triple:     "x86_64-pc-windows-msvc",
				assetName:  "codex-x86_64-pc-windows-msvc.exe.zip",
				archive:    archiveZip,
				binaryName: "codex-x86_64-pc-windows-msvc.exe",
				exeName:    "codex.exe",
			}, true
		case "arm64":
			return targetInfo{
				triple:     "aarch64-pc-windows-msvc",
				assetName:  "codex-aarch64-pc-windows-msvc.exe.zip",
				archive:    archiveZip,
				binaryName: "codex-aarch64-pc-windows-msvc.exe",
				exeName:    "codex.exe",
			}, true
		}
	}
	return targetInfo{}, false
}

func ensureBundledBinary(cfg bundleConfig) (string, error) {
	info, ok := detectTarget(runtimeGOOS, runtimeGOARCH)
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtimeGOOS, runtimeGOARCH)
	}

	cacheDir, err := cfg.cacheDirPath()
	if err != nil {
		return "", err
	}

	release := cfg.releaseTagName()
	checksumHex, err := cfg.checksumValue()
	if err != nil {
		return "", fmt.Errorf("resolve checksum: %w", err)
	}
	targetDir := filepath.Join(cacheDir, release, info.triple)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create bundle directory: %w", err)
	}

	destPath := filepath.Join(targetDir, info.exeName)
	if statErr := ensureBinaryState(destPath); statErr == nil {
		if checksumHex == "" {
			return destPath, nil
		}
		if err := verifyChecksum(destPath, checksumHex); err == nil {
			return destPath, nil
		} else if errors.Is(err, ErrChecksumMismatch) {
			_ = os.Remove(destPath)
		} else {
			return "", fmt.Errorf("verify cached binary: %w", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat bundled binary: %w", statErr)
	}

	if err := downloadBinaryFunc(info, release, destPath); err != nil {
		return "", err
	}
	if checksumHex != "" {
		if err := verifyChecksum(destPath, checksumHex); err != nil {
			_ = os.Remove(destPath)
			return "", fmt.Errorf("verify downloaded binary: %w", err)
		}
	}
	return destPath, nil
}

func ensureBinaryState(path string) error {
	_, err := os.Stat(path)
	return err
}

func downloadBinaryFromRelease(info targetInfo, release, destPath string) error {
	url := fmt.Sprintf("https://github.com/openai/codex/releases/download/%s/%s", release, info.assetName)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download codex binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download codex binary: unexpected status %d", resp.StatusCode)
	}

	switch info.archive {
	case archiveTarGz:
		return extractTarGzBinary(resp.Body, info, destPath)
	case archiveZip:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read zip archive: %w", err)
		}
		return extractZipBinary(data, info, destPath)
	default:
		return fmt.Errorf("unsupported archive for %s", info.assetName)
	}
}

func extractTarGzBinary(r io.Reader, info targetInfo, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != info.binaryName {
			continue
		}
		return writeBinary(tr, destPath)
	}
	return fmt.Errorf("binary %s not found in archive", info.binaryName)
}

func extractZipBinary(data []byte, info targetInfo, destPath string) error {
	readerAt := bytes.NewReader(data)
	zr, err := zip.NewReader(readerAt, int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	for _, file := range zr.File {
		if filepath.Base(file.Name) != info.binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		err = writeBinary(rc, destPath)
		rc.Close()
		return err
	}
	return fmt.Errorf("binary %s not found in archive", info.binaryName)
}

func verifyChecksum(path, expectedHex string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open binary for checksum: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash binary: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedHex, actual)
	}
	return nil
}

func writeBinary(r io.Reader, destPath string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), filepath.Base(destPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	if err := tmpFile.Chmod(0o700); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return fmt.Errorf("chmod temp binary: %w", err)
	}
	tmpPath := tmpFile.Name()
	f := tmpFile
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write binary: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close binary: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename binary: %w", err)
	}
	return nil
}
