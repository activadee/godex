package godex

import (
	"context"
	"errors"
	"testing"
)

func TestThreadRunReturnsThreadStreamError(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: threadErrorEvents(t)}}}
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

func TestRunStreamedResultWaitReturnsThreadStreamError(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: threadErrorEvents(t)}}}
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
