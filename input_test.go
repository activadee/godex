package godex

import "testing"

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
