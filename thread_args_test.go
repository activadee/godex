package godex

import (
	"context"
	"os"
	"testing"
)

func TestThreadRunForwardsThreadOptions(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: successEvents(t)}}}
	threadOpts := ThreadOptions{
		Model:            "gpt-test-1",
		SandboxMode:      SandboxModeWorkspaceWrite,
		WorkingDirectory: "/tmp/workspace",
		SkipGitRepoCheck: true,
	}
	thread := newThread(runner, CodexOptions{}, threadOpts, "")

	result, err := thread.Run(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.FinalResponse != "Hello" {
		t.Fatalf("unexpected final response %q", result.FinalResponse)
	}

	call := runner.lastCall()
	if call.Model != threadOpts.Model {
		t.Fatalf("expected model %q, got %q", threadOpts.Model, call.Model)
	}
	if call.SandboxMode != string(threadOpts.SandboxMode) {
		t.Fatalf("expected sandbox %q, got %q", threadOpts.SandboxMode, call.SandboxMode)
	}
	if call.WorkingDirectory != threadOpts.WorkingDirectory {
		t.Fatalf("expected working directory %q, got %q", threadOpts.WorkingDirectory, call.WorkingDirectory)
	}
	if !call.SkipGitRepoCheck {
		t.Fatalf("expected skipGitRepoCheck to be true")
	}
}

func TestThreadRunReusesThreadIDForSubsequentCalls(t *testing.T) {
	batches := []fakeRun{
		{events: successEvents(t)},
		{events: successEvents(t)},
	}
	runner := &fakeRunner{t: t, batches: batches}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	if _, err := thread.Run(context.Background(), "first", nil); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if thread.ID() != "thread_1" {
		t.Fatalf("expected thread ID to be set to thread_1, got %q", thread.ID())
	}

	if _, err := thread.Run(context.Background(), "second", nil); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	firstCall := runner.callAt(0)
	if firstCall.ThreadID != "" {
		t.Fatalf("expected first call thread id to be empty, got %q", firstCall.ThreadID)
	}
	secondCall := runner.callAt(1)
	if secondCall.ThreadID != "thread_1" {
		t.Fatalf("expected second call to resume thread_1, got %q", secondCall.ThreadID)
	}
}

func TestThreadRunStreamedCleansOutputSchemaFile(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: successEvents(t)}}}
	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
	}

	result, err := thread.RunStreamed(context.Background(), "structured", &TurnOptions{OutputSchema: schema})
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}

	for range result.Events() {
		// drain events
	}
	if err := result.Wait(); err != nil {
		t.Fatalf("result.Wait returned error: %v", err)
	}

	call := runner.lastCall()
	if call.OutputSchemaPath == "" {
		t.Fatal("expected OutputSchemaPath to be set")
	}
	if _, statErr := os.Stat(call.OutputSchemaPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected schema file to be cleaned up, stat error: %v", statErr)
	}
}
