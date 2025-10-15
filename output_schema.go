package godex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func createOutputSchemaFile(schema any) (string, func() error, error) {
	noCleanup := func() error { return nil }
	if schema == nil {
		return "", noCleanup, nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return "", noCleanup, fmt.Errorf("marshal output schema: %w", err)
	}
	if len(data) == 0 || data[0] != '{' {
		return "", noCleanup, errors.New("output schema must serialize to a JSON object")
	}

	dir, err := os.MkdirTemp("", "codex-output-schema-")
	if err != nil {
		return "", noCleanup, fmt.Errorf("create schema temp dir: %w", err)
	}

	cleanup := func() error {
		return os.RemoveAll(dir)
	}

	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		_ = cleanup()
		return "", noCleanup, fmt.Errorf("write schema file: %w", err)
	}

	return path, cleanup, nil
}
