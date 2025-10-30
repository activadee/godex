package godex

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestThreadRunReturnsThreadStreamError(t *testing.T) {
	runner := &fakeRunner{t: t, events: threadErrorEvents(t)}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	_, err := thread.Run(context.Background(), "trigger error", nil)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	var streamErr *ThreadStreamError
	if !errors.As(err, &streamErr) {
		t.Fatalf("expected ThreadStreamError, got %T", err)
	}
	if streamErr.Message != "boom" {
		t.Fatalf("unexpected message %q", streamErr.Message)
	}
}

func threadErrorEvents(t *testing.T) [][]byte {
	events := []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "error", "message": "boom"},
	}
	var encoded [][]byte
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		encoded = append(encoded, data)
	}
	return encoded
}

func TestRunStreamedResultWaitReturnsThreadStreamError(t *testing.T) {
	runner := &fakeRunner{t: t, events: threadErrorEvents(t)}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	result, err := thread.RunStreamed(context.Background(), "trigger error", nil)
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	defer result.Close()

	for range result.Events() {
		// drain events
	}

	if err := result.Wait(); err == nil {
		t.Fatal("expected error but got nil")
	} else {
		var streamErr *ThreadStreamError
		if !errors.As(err, &streamErr) {
			t.Fatalf("expected ThreadStreamError, got %T", err)
		}
	}
}
