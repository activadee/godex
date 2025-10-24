package godex

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/activadee/godex/internal/codexexec"
)

type fakeRunner struct {
	t      *testing.T
	mu     sync.Mutex
	calls  []codexexec.Args
	events [][]byte
	runErr error
}

func (f *fakeRunner) Run(ctx context.Context, args codexexec.Args, handleLine func([]byte) error) error {
	f.mu.Lock()
	f.calls = append(f.calls, args)
	f.mu.Unlock()
	for _, event := range f.events {
		if err := handleLine(event); err != nil {
			return err
		}
	}
	return f.runErr
}

func (f *fakeRunner) lastCall() codexexec.Args {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		f.t.Fatalf("fakeRunner expected at least one call")
	}
	return f.calls[len(f.calls)-1]
}

func TestThreadRunInputsForwardsImages(t *testing.T) {
	runner := &fakeRunner{t: t, events: successEvents(t)}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")
	segments := []InputSegment{
		TextSegment("Describe the images"),
		LocalImageSegment("/tmp/one.png"),
		TextSegment("Focus on differences"),
		LocalImageSegment("/tmp/two.png"),
	}

	result, err := thread.RunInputs(context.Background(), segments, nil)
	if err != nil {
		t.Fatalf("RunInputs returned error: %v", err)
	}
	if result.FinalResponse != "Hello" {
		t.Fatalf("unexpected final response %q", result.FinalResponse)
	}

	call := runner.lastCall()
	if want := "Describe the images\n\nFocus on differences"; call.Input != want {
		t.Fatalf("expected prompt %q, got %q", want, call.Input)
	}
	if len(call.Images) != 2 || call.Images[0] != "/tmp/one.png" || call.Images[1] != "/tmp/two.png" {
		t.Fatalf("unexpected images slice: %v", call.Images)
	}
}

func successEvents(t *testing.T) [][]byte {
	events := []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "item.completed", "item": map[string]any{"id": "item_1", "type": "agent_message", "text": "Hello"}},
		{"type": "turn.completed", "usage": map[string]any{"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
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
