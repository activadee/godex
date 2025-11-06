package godex

import (
	"context"
	"sync"
	"testing"
)

func TestThreadRunStreamedReturnsEvents(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: successEvents(t)}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	result, err := thread.RunStreamed(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	defer result.Close()

	var eventTypes []ThreadEventType
	for event := range result.Events() {
		eventTypes = append(eventTypes, event.EventType())
	}
	if err := result.Wait(); err != nil {
		t.Fatalf("result.Wait returned error: %v", err)
	}

	expected := []ThreadEventType{ThreadEventTypeThreadStarted, ThreadEventTypeItemCompleted, ThreadEventTypeTurnCompleted}
	if len(eventTypes) != len(expected) {
		t.Fatalf("expected %d events, got %d", len(expected), len(eventTypes))
	}
	for i, typ := range expected {
		if eventTypes[i] != typ {
			t.Fatalf("event %d: expected %s, got %s", i, typ, eventTypes[i])
		}
	}
}

func TestThreadRunStreamedInputsForwardsImages(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: successEvents(t)}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	segments := []InputSegment{
		TextSegment("Describe the images"),
		LocalImageSegment("/tmp/one.png"),
		TextSegment("Focus on differences"),
		LocalImageSegment("/tmp/two.png"),
	}

	result, err := thread.RunStreamedInputs(context.Background(), segments, nil)
	if err != nil {
		t.Fatalf("RunStreamedInputs returned error: %v", err)
	}
	defer result.Close()

	for range result.Events() {
		// drain events
	}
	if err := result.Wait(); err != nil {
		t.Fatalf("result.Wait returned error: %v", err)
	}

	call := runner.lastCall()
	if want := "Describe the images\n\nFocus on differences"; call.Input != want {
		t.Fatalf("expected prompt %q, got %q", want, call.Input)
	}
	if len(call.Images) != 2 || call.Images[0] != "/tmp/one.png" || call.Images[1] != "/tmp/two.png" {
		t.Fatalf("unexpected images slice: %v", call.Images)
	}
}

func TestStreamCallbacksDispatchTypedItems(t *testing.T) {
	events := marshalEvents(t, []map[string]any{
		{"type": "thread.started", "thread_id": "thread_1"},
		{"type": "item.updated", "item": map[string]any{
			"id":   "message_1",
			"type": "agent_message",
			"text": "partial: hello",
		}},
		{"type": "item.updated", "item": map[string]any{
			"id":                "command_1",
			"type":              "command_execution",
			"command":           "go test ./...",
			"aggregated_output": "running tests",
			"status":            "in_progress",
		}},
		{"type": "item.completed", "item": map[string]any{
			"id":     "patch_1",
			"type":   "file_change",
			"status": "completed",
			"changes": []map[string]any{
				{"path": "main.go", "kind": "update"},
				{"path": "README.md", "kind": "update"},
			},
		}},
		{"type": "item.completed", "item": map[string]any{
			"id":    "search_1",
			"type":  "web_search",
			"query": "godex callbacks",
		}},
		{"type": "turn.completed", "usage": map[string]any{"input_tokens": 1, "cached_input_tokens": 0, "output_tokens": 1}},
	})

	runner := &fakeRunner{t: t, batches: []fakeRun{{events: events}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	var (
		mu          sync.Mutex
		messages    []StreamMessageEvent
		commands    []StreamCommandEvent
		patches     []StreamPatchEvent
		fileChanges []StreamFileChangeEvent
		webSearches []StreamWebSearchEvent
	)

	callbacks := &StreamCallbacks{
		OnMessage: func(evt StreamMessageEvent) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, evt)
		},
		OnCommand: func(evt StreamCommandEvent) {
			mu.Lock()
			defer mu.Unlock()
			commands = append(commands, evt)
		},
		OnPatch: func(evt StreamPatchEvent) {
			mu.Lock()
			defer mu.Unlock()
			patches = append(patches, evt)
		},
		OnFileChange: func(evt StreamFileChangeEvent) {
			mu.Lock()
			defer mu.Unlock()
			fileChanges = append(fileChanges, evt)
		},
		OnWebSearch: func(evt StreamWebSearchEvent) {
			mu.Lock()
			defer mu.Unlock()
			webSearches = append(webSearches, evt)
		},
	}

	result, err := thread.RunStreamed(context.Background(), "callbacks please", &TurnOptions{Callbacks: callbacks})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	defer result.Close()

	for range result.Events() {
		// Drain events while callbacks handle type-specific logic.
	}

	if err := result.Wait(); err != nil {
		t.Fatalf("result.Wait returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(messages) != 1 {
		t.Fatalf("expected 1 message callback, got %d", len(messages))
	}
	if messages[0].Stage != StreamItemStageUpdated || messages[0].Message.Text != "partial: hello" {
		t.Fatalf("unexpected message callback payload: %+v", messages[0])
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command callback, got %d", len(commands))
	}
	if commands[0].Stage != StreamItemStageUpdated || commands[0].Command.Command != "go test ./..." {
		t.Fatalf("unexpected command callback payload: %+v", commands[0])
	}

	if len(patches) != 1 {
		t.Fatalf("expected 1 patch callback, got %d", len(patches))
	}
	if patches[0].Stage != StreamItemStageCompleted || patches[0].Patch.ID != "patch_1" {
		t.Fatalf("unexpected patch callback payload: %+v", patches[0])
	}

	if len(fileChanges) != 2 {
		t.Fatalf("expected 2 file change callbacks, got %d", len(fileChanges))
	}
	if fileChanges[0].Patch.ID != "patch_1" || fileChanges[0].Change.Path != "main.go" {
		t.Fatalf("unexpected first file change payload: %+v", fileChanges[0])
	}
	if fileChanges[1].Patch.ID != "patch_1" || fileChanges[1].Change.Path != "README.md" {
		t.Fatalf("unexpected second file change payload: %+v", fileChanges[1])
	}
	for _, change := range fileChanges {
		if change.Stage != StreamItemStageCompleted {
			t.Fatalf("expected completed stage for file change, got %+v", change.Stage)
		}
	}

	if len(webSearches) != 1 {
		t.Fatalf("expected 1 web search callback, got %d", len(webSearches))
	}
	if webSearches[0].Stage != StreamItemStageCompleted || webSearches[0].Search.Query != "godex callbacks" {
		t.Fatalf("unexpected web search callback payload: %+v", webSearches[0])
	}
}
