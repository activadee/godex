package godex

import (
	"os"
	"strings"
	"testing"
)

func TestDecodeThreadEventItemCompleted(t *testing.T) {
	raw := []byte(`
{
  "type": "item.completed",
  "item": {
    "id": "item_1",
    "type": "agent_message",
    "text": "Hello, world!"
  }
}`)
	event, err := decodeThreadEvent(raw)
	if err != nil {
		t.Fatalf("decodeThreadEvent returned error: %v", err)
	}

	completed, ok := event.(ItemCompletedEvent)
	if !ok {
		t.Fatalf("expected ItemCompletedEvent, got %T", event)
	}

	message, ok := completed.Item.(AgentMessageItem)
	if !ok {
		t.Fatalf("expected AgentMessageItem, got %T", completed.Item)
	}

	if message.Text != "Hello, world!" {
		t.Fatalf("unexpected message text %q", message.Text)
	}
}

func TestDecodeThreadEventThreadStarted(t *testing.T) {
	raw := []byte(`{"type":"thread.started","thread_id":"thread_123"}`)
	event, err := decodeThreadEvent(raw)
	if err != nil {
		t.Fatalf("decodeThreadEvent returned error: %v", err)
	}
	started, ok := event.(ThreadStartedEvent)
	if !ok {
		t.Fatalf("expected ThreadStartedEvent, got %T", event)
	}
	if started.ThreadID != "thread_123" {
		t.Fatalf("unexpected thread id %q", started.ThreadID)
	}
}

func TestCreateOutputSchemaFile(t *testing.T) {
	path, cleanup, err := createOutputSchemaFile(map[string]any{
		"type": "object",
	})
	if err != nil {
		t.Fatalf("createOutputSchemaFile returned error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unable to read schema file: %v", err)
	}

	if !strings.Contains(string(data), `"type":"object"`) {
		t.Fatalf("schema file did not contain expected contents: %s", string(data))
	}
}

func TestCreateOutputSchemaFileRejectsNonObject(t *testing.T) {
	if _, _, err := createOutputSchemaFile([]string{"not", "object"}); err == nil {
		t.Fatal("expected error for non-object schema but received none")
	}
}
