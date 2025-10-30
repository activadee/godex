package godex

import (
	"context"
	"testing"
)

func TestThreadRunInputsForwardsImages(t *testing.T) {
	runner := &fakeRunner{t: t, batches: []fakeRun{{events: successEvents(t)}}}
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
