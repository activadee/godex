package godex

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/activadee/godex/internal/codexexec"
)

type fakeRun struct {
	events [][]byte
	err    error
}

type fakeRunner struct {
	t        *testing.T
	mu       sync.Mutex
	calls    []codexexec.Args
	batches  []fakeRun
	defaults fakeRun
}

func (f *fakeRunner) Run(ctx context.Context, args codexexec.Args, handleLine func([]byte) error) error {
	_ = ctx

	f.mu.Lock()
	f.calls = append(f.calls, args)
	var batch fakeRun
	if len(f.batches) > 0 {
		batch = f.batches[0]
		f.batches = f.batches[1:]
	} else {
		batch = f.defaults
	}
	f.mu.Unlock()

	for _, event := range batch.events {
		if err := handleLine(event); err != nil {
			return err
		}
	}
	return batch.err
}

func (f *fakeRunner) lastCall() codexexec.Args {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		f.t.Fatalf("fakeRunner expected at least one call")
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeRunner) callAt(index int) codexexec.Args {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.calls) {
		f.t.Fatalf("fakeRunner call index %d out of range", index)
	}
	return f.calls[index]
}

func successEvents(t *testing.T) [][]byte {
	return marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "item.completed", "item": map[string]any{"id": "item_1", "type": "agent_message", "text": "Hello"}},
		{"type": "turn.completed", "usage": map[string]any{"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
	})
}

func threadErrorEvents(t *testing.T) [][]byte {
	return marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "error", "message": "boom"},
	})
}

func marshalEvents(t *testing.T, events []map[string]any) [][]byte {
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
