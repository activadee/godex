package godex

import (
	"fmt"
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

type normalizedInput struct {
	prompt string
	images []string
}

func normalizeInput(base string, segments []InputSegment) (normalizedInput, error) {
	if len(segments) == 0 {
		return normalizedInput{prompt: base}, nil
	}

	var (
		promptParts []string
		images      []string
	)

	for i, segment := range segments {
		hasText := segment.Text != ""
		hasImage := segment.LocalImagePath != ""

		switch {
		case hasText && hasImage:
			return normalizedInput{}, fmt.Errorf("input segment %d must specify either text or image, not both", i)
		case !hasText && !hasImage:
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

	return normalizedInput{prompt: prompt, images: images}, nil
}
