package godex

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNormalizeInputUsesBaseWhenNoSegments(t *testing.T) {
	prepared, err := normalizeInput("hello", nil)
	if err != nil {
		t.Fatalf("normalizeInput returned error: %v", err)
	}
	if prepared.prompt != "hello" {
		t.Fatalf("expected prompt to equal base input, got %q", prepared.prompt)
	}
	if len(prepared.images) != 0 {
		t.Fatalf("expected no images, got %v", prepared.images)
	}
}

func TestNormalizeInputJoinsTextSegments(t *testing.T) {
	segments := []InputSegment{
		TextSegment("first"),
		TextSegment("second"),
	}
	prepared, err := normalizeInput("base", segments)
	if err != nil {
		t.Fatalf("normalizeInput returned error: %v", err)
	}
	expected := "first\n\nsecond"
	if prepared.prompt != expected {
		t.Fatalf("expected prompt %q, got %q", expected, prepared.prompt)
	}
	if len(prepared.images) != 0 {
		t.Fatalf("expected no images, got %v", prepared.images)
	}
}

func TestNormalizeInputCollectsImages(t *testing.T) {
	segments := []InputSegment{
		LocalImageSegment("/tmp/a.png"),
		LocalImageSegment("/tmp/b.png"),
	}
	prepared, err := normalizeInput("", segments)
	if err != nil {
		t.Fatalf("normalizeInput returned error: %v", err)
	}
	if prepared.prompt != "" {
		t.Fatalf("expected empty prompt, got %q", prepared.prompt)
	}
	if len(prepared.images) != 2 || prepared.images[0] != "/tmp/a.png" || prepared.images[1] != "/tmp/b.png" {
		t.Fatalf("unexpected images slice: %v", prepared.images)
	}
}

func TestNormalizeInputRejectsInvalidSegments(t *testing.T) {
	_, err := normalizeInput("", []InputSegment{{}})
	if err == nil {
		t.Fatal("expected error for empty segment, got nil")
	}

	_, err = normalizeInput("", []InputSegment{{Text: "text", LocalImagePath: "path"}})
	if err == nil {
		t.Fatal("expected error when both text and image are set")
	}
}

func TestURLImageSegmentDownloadsAndCleansUp(t *testing.T) {
	imageData := decodeBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4//8/AAX+Av7l/wAAAABJRU5ErkJggg==")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData)
	}))
	defer server.Close()

	segment, err := URLImageSegment(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("URLImageSegment returned error: %v", err)
	}
	if segment.LocalImagePath == "" {
		t.Fatal("expected LocalImagePath to be set")
	}

	prepared, err := normalizeInput("", []InputSegment{segment})
	if err != nil {
		t.Fatalf("normalizeInput returned error: %v", err)
	}
	if len(prepared.images) != 1 {
		t.Fatalf("expected one image, got %v", prepared.images)
	}

	path := prepared.images[0]
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected image file to exist: %v", err)
	}

	prepared.cleanup()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected image file to be cleaned up, got %v", err)
	}
}

func TestURLImageSegmentRejectsNonImageContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()

	if _, err := URLImageSegment(context.Background(), server.URL); err == nil {
		t.Fatal("expected error for non-image content type")
	}
}

func TestURLImageSegmentRejectsOversizedImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := io.CopyN(w, zeroReader{}, int64(maxURLImageSizeBytes)+1); err != nil && err != io.EOF {
			t.Fatalf("failed to write large body: %v", err)
		}
	}))
	defer server.Close()

	_, err := URLImageSegment(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for oversized image")
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestBytesImageSegmentCreatesFileWithExtension(t *testing.T) {
	imageData := decodeBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4//8/AAX+Av7l/wAAAABJRU5ErkJggg==")

	segment, err := BytesImageSegment("example", imageData)
	if err != nil {
		t.Fatalf("BytesImageSegment returned error: %v", err)
	}
	if segment.LocalImagePath == "" {
		t.Fatal("expected LocalImagePath to be set")
	}
	if !strings.HasSuffix(segment.LocalImagePath, ".png") {
		t.Fatalf("expected .png extension, got %q", segment.LocalImagePath)
	}

	prepared, err := normalizeInput("", []InputSegment{segment})
	if err != nil {
		t.Fatalf("normalizeInput returned error: %v", err)
	}
	if len(prepared.images) != 1 {
		t.Fatalf("expected one image, got %v", prepared.images)
	}

	path := prepared.images[0]
	prepared.cleanup()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected image file to be cleaned up, got %v", err)
	}
}

func decodeBase64(t *testing.T, s string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	return data
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
