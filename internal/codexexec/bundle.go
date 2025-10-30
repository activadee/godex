package codexexec

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type archiveKind int

const (
	archiveTarGz archiveKind = iota
	archiveZip
)

const defaultCodexReleaseTag = "rust-v0.52.0"

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

func releaseTag() string {
	if v := os.Getenv("GODEX_CLI_RELEASE_TAG"); v != "" {
		return v
	}
	return defaultCodexReleaseTag
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

func ensureBundledBinary() (string, error) {
	info, ok := detectTarget(runtimeGOOS, runtimeGOARCH)
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtimeGOOS, runtimeGOARCH)
	}

	cacheDir, err := bundleCacheDir()
	if err != nil {
		return "", err
	}

	release := releaseTag()
	targetDir := filepath.Join(cacheDir, release, info.triple)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create bundle directory: %w", err)
	}

	destPath := filepath.Join(targetDir, info.exeName)
	if statErr := ensureBinaryState(destPath); statErr == nil {
		return destPath, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat bundled binary: %w", statErr)
	}

	if err := downloadBinaryFunc(info, release, destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func ensureBinaryState(path string) error {
	_, err := os.Stat(path)
	return err
}

func bundleCacheDir() (string, error) {
	if override := os.Getenv("GODEX_CLI_CACHE"); override != "" {
		return override, nil
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "godex", "codex"), nil
	}
	return filepath.Join(os.TempDir(), "godex", "codex"), nil
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
