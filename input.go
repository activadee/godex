package godex

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// InputSegment represents a piece of user-provided input sent to the Codex CLI.
// Exactly one of Text or LocalImagePath must be populated.
type InputSegment struct {
	// Text holds a natural-language prompt fragment. Leave empty to indicate the
	// segment references an image instead.
	Text string

	// LocalImagePath contains a filesystem path to an image that should be
	// forwarded to the CLI via --image. Leave empty for text segments.
	LocalImagePath string

	cleanup func()
}

// TextSegment creates a textual input segment. Multiple text segments are
// concatenated with blank lines between them, matching the TypeScript SDK's
// behaviour.
func TextSegment(text string) InputSegment {
	return InputSegment{Text: text}
}

// LocalImageSegment creates an input segment pointing at a local image file.
// The path is forwarded to the Codex CLI using repeated --image flags.
func LocalImageSegment(path string) InputSegment {
	return InputSegment{LocalImagePath: path}
}

const (
	maxURLImageSizeBytes = 8 << 20 // 8 MiB safety limit for remote downloads
	sniffBufferSize      = 512
)

// URLImageSegment downloads an image from the provided URL into a temporary file and
// returns an input segment that references it. The file is cleaned up automatically
// when the run finishes.
func URLImageSegment(ctx context.Context, rawURL string) (InputSegment, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return InputSegment{}, fmt.Errorf("create image request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return InputSegment{}, fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return InputSegment{}, fmt.Errorf("download image: unexpected status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return InputSegment{}, fmt.Errorf("download image: missing Content-Type header")
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return InputSegment{}, fmt.Errorf("parse Content-Type %q: %w", contentType, err)
	}
	if !strings.HasPrefix(mediaType, "image/") {
		return InputSegment{}, fmt.Errorf("download image: content-type %q is not an image", mediaType)
	}

	ext := extensionForMediaType(mediaType)
	limited := &io.LimitedReader{R: resp.Body, N: maxURLImageSizeBytes + 1}
	sniff := make([]byte, sniffBufferSize)
	n, err := io.ReadFull(limited, sniff)
	switch {
	case err == io.EOF && n == 0:
		return InputSegment{}, fmt.Errorf("download image: empty response body")
	case err != nil && err != io.ErrUnexpectedEOF:
		return InputSegment{}, fmt.Errorf("read image body: %w", err)
	}

	if ext == "" && n > 0 {
		detected := http.DetectContentType(sniff[:n])
		if strings.HasPrefix(detected, "image/") {
			ext = extensionForMediaType(detected)
		}
	}

	path, cleanup, err := writeTempImageStream(ext, sniff[:n], limited, maxURLImageSizeBytes)
	if err != nil {
		return InputSegment{}, err
	}

	return InputSegment{LocalImagePath: path, cleanup: cleanup}, nil
}

// BytesImageSegment writes the provided image bytes to a temporary file and returns
// a segment that references it. The file is cleaned up automatically when the run finishes.
func BytesImageSegment(name string, data []byte) (InputSegment, error) {
	if len(data) == 0 {
		return InputSegment{}, fmt.Errorf("image data is empty")
	}

	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(name)))
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	mediaType := ""
	if ext != "" {
		mediaType = mime.TypeByExtension(ext)
	}
	detected := http.DetectContentType(data)
	if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
		mediaType = detected
	}

	if !strings.HasPrefix(mediaType, "image/") {
		return InputSegment{}, fmt.Errorf("bytes content-type %q is not an image", mediaType)
	}

	if ext == "" {
		ext = extensionForMediaType(mediaType)
	}

	return newTempImageSegment(data, ext)
}

type normalizedInput struct {
	prompt  string
	images  []string
	cleanup func()
}

func normalizeInput(base string, segments []InputSegment) (normalizedInput, error) {
	noCleanup := func() {}

	if len(segments) == 0 {
		return normalizedInput{prompt: base, cleanup: noCleanup}, nil
	}

	var (
		promptParts []string
		images      []string
		cleanups    []func()
	)

	cleanupAll := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			if cleanups[i] != nil {
				cleanups[i]()
			}
		}
	}

	for i, segment := range segments {
		if segment.cleanup != nil {
			cleanups = append(cleanups, segment.cleanup)
		}

		hasText := segment.Text != ""
		hasImage := segment.LocalImagePath != ""

		switch {
		case hasText && hasImage:
			cleanupAll()
			return normalizedInput{}, fmt.Errorf("input segment %d must specify either text or image, not both", i)
		case !hasText && !hasImage:
			cleanupAll()
			return normalizedInput{}, fmt.Errorf("input segment %d must specify text or image", i)
		case hasText:
			promptParts = append(promptParts, segment.Text)
		case hasImage:
			images = append(images, segment.LocalImagePath)
		}
	}

	prompt := base
	if len(promptParts) > 0 {
		prompt = strings.Join(promptParts, "\n\n")
	}

	return normalizedInput{prompt: prompt, images: images, cleanup: cleanupAll}, nil
}

func newTempImageSegment(data []byte, ext string) (InputSegment, error) {
	path, cleanup, err := writeTempImageBytes(ext, data)
	if err != nil {
		return InputSegment{}, err
	}
	return InputSegment{LocalImagePath: path, cleanup: cleanup}, nil
}

func writeTempImageBytes(ext string, data []byte) (string, func(), error) {
	return writeTempImageFile(ext, func(f *os.File) (int64, error) {
		n, err := f.Write(data)
		return int64(n), err
	})
}

func writeTempImageStream(ext string, head []byte, body io.Reader, maxSize int64) (string, func(), error) {
	validator := func(total int64) error {
		if total == 0 {
			return fmt.Errorf("download image: empty response body")
		}
		if total > maxSize {
			return fmt.Errorf("download image: exceeded %d byte size limit", maxSize)
		}
		return nil
	}

	return writeTempImageFile(ext, func(f *os.File) (int64, error) {
		var total int64
		if len(head) > 0 {
			n, err := f.Write(head)
			total += int64(n)
			if err != nil {
				return total, err
			}
		}
		written, err := io.Copy(f, body)
		total += written
		return total, err
	}, validator)
}

func writeTempImageFile(ext string, writer func(*os.File) (int64, error), validators ...func(int64) error) (string, func(), error) {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	pattern := "codex-image-*"
	if ext != "" {
		pattern += ext
	}

	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create temp image: %w", err)
	}

	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}

	total, err := writer(file)
	if err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write temp image: %w", err)
	}

	for _, validate := range validators {
		if validate == nil {
			continue
		}
		if err := validate(total); err != nil {
			_ = file.Close()
			cleanup()
			return "", nil, err
		}
	}

	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp image: %w", err)
	}

	return path, cleanup, nil
}

func extensionForMediaType(mediaType string) string {
	if mediaType == "" {
		return ""
	}

	exts, _ := mime.ExtensionsByType(mediaType)
	if len(exts) == 0 {
		return ""
	}

	for _, preferred := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg"} {
		for _, candidate := range exts {
			if candidate == preferred {
				return candidate
			}
		}
	}

	return exts[0]
}
