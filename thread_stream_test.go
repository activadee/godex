package godex

import (
	"context"
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
