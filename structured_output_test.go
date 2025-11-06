package godex

import (
	"context"
	"errors"
	"testing"
)

type structuredUpdate struct {
	Headline string `json:"headline"`
	NextStep string `json:"next_step"`
}

func TestRunJSONReturnsTypedValue(t *testing.T) {
	events := marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "item.completed", "item": map[string]any{
			"id":   "msg_1",
			"type": "agent_message",
			"text": `{"headline":"Release ready","next_step":"Ship it"}`,
		}},
		{"type": "turn.completed", "usage": map[string]any{"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
	})

	runner := &fakeRunner{t: t, batches: []fakeRun{{events: events}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	update, err := RunJSON[structuredUpdate](context.Background(), thread, "structured", nil)
	if err != nil {
		t.Fatalf("RunJSON returned error: %v", err)
	}

	if update.Headline != "Release ready" || update.NextStep != "Ship it" {
		t.Fatalf("unexpected update: %+v", update)
	}

	if call := runner.lastCall(); call.OutputSchemaPath == "" {
		t.Fatal("expected OutputSchemaPath to be set")
	}
}

func TestRunJSONSchemaViolation(t *testing.T) {
	events := marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "turn.failed", "error": map[string]any{"message": "Structured output schema violation: missing property 'headline'"}},
	})

	runner := &fakeRunner{t: t, batches: []fakeRun{{events: events}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	_, err := RunJSON[structuredUpdate](context.Background(), thread, "structured", nil)
	if err == nil {
		t.Fatal("expected RunJSON to return error")
	}

	var schemaErr *SchemaViolationError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("expected SchemaViolationError, got %T", err)
	}
	if schemaErr.Message == "" {
		t.Fatal("expected schema error message to be populated")
	}
}

func TestRunJSONRequiresSchemaWhenInferenceDisabled(t *testing.T) {
	thread := newThread(&fakeRunner{t: t}, CodexOptions{}, ThreadOptions{}, "")

	_, err := RunJSON[structuredUpdate](context.Background(), thread, "structured", &RunJSONOptions[structuredUpdate]{
		DisableSchemaInference: true,
	})
	if err == nil {
		t.Fatal("expected RunJSON to fail without schema when inference disabled")
	}
}

func TestRunStreamedJSONEmitsUpdates(t *testing.T) {
	events := marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "item.updated", "item": map[string]any{
			"id":   "msg_1",
			"type": "agent_message",
			"text": `{"headline":"Draft message","next_step":"Review"}`,
		}},
		{"type": "item.completed", "item": map[string]any{
			"id":   "msg_1",
			"type": "agent_message",
			"text": `{"headline":"Final headline","next_step":"Publish"}`,
		}},
		{"type": "turn.completed", "usage": map[string]any{"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
	})

	runner := &fakeRunner{t: t, batches: []fakeRun{{events: events}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	result, err := RunStreamedJSON[structuredUpdate](context.Background(), thread, "structured", nil)
	if err != nil {
		t.Fatalf("RunStreamedJSON returned error: %v", err)
	}
	defer result.Close()

	eventDone := make(chan struct{})
	go func() {
		for range result.Events() {
		}
		close(eventDone)
	}()

	var updates []RunStreamedJSONUpdate[structuredUpdate]
	for update := range result.Updates() {
		updates = append(updates, update)
	}

	<-eventDone

	if err := result.Wait(); err != nil {
		t.Fatalf("result.Wait returned error: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].Final {
		t.Fatal("expected first update to be non-final")
	}
	if !updates[1].Final {
		t.Fatal("expected second update to be final")
	}
	if updates[1].Value.Headline != "Final headline" || updates[1].Value.NextStep != "Publish" {
		t.Fatalf("unexpected final update: %+v", updates[1].Value)
	}
}

func TestRunStreamedJSONSchemaViolation(t *testing.T) {
	events := marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "turn.failed", "error": map[string]any{"message": "structured output schema violation: headline missing"}},
	})

	runner := &fakeRunner{t: t, batches: []fakeRun{{events: events}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	result, err := RunStreamedJSON[structuredUpdate](context.Background(), thread, "structured", nil)
	if err != nil {
		t.Fatalf("RunStreamedJSON returned error: %v", err)
	}
	defer result.Close()

	eventDone := make(chan struct{})
	go func() {
		for range result.Events() {
		}
		close(eventDone)
	}()

	for range result.Updates() {
		// drain updates
	}

	<-eventDone

	waitErr := result.Wait()
	if waitErr == nil {
		t.Fatal("expected Wait to return error")
	}

	var schemaErr *SchemaViolationError
	if !errors.As(waitErr, &schemaErr) {
		t.Fatalf("expected SchemaViolationError, got %T", waitErr)
	}
	if schemaErr.Message == "" {
		t.Fatal("expected schema error message to be populated")
	}
}
