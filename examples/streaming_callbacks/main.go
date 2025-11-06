package main

import (
	"context"
	"fmt"
	"log"

	"github.com/activadee/godex"
)

func main() {
	client, err := godex.New(godex.CodexOptions{})
	if err != nil {
		log.Fatalf("create codex client: %v", err)
	}

	thread := client.StartThread(godex.ThreadOptions{
		Model: "gpt-5",
	})

	callbacks := &godex.StreamCallbacks{
		OnMessage: func(evt godex.StreamMessageEvent) {
			switch evt.Stage {
			case godex.StreamItemStageUpdated:
				fmt.Printf("[assistant partial] %s\n", evt.Message.Text)
			case godex.StreamItemStageCompleted:
				fmt.Printf("[assistant final] %s\n", evt.Message.Text)
			}
		},
		OnCommand: func(evt godex.StreamCommandEvent) {
			fmt.Printf("[command %s] %s\n", evt.Command.Status, evt.Command.Command)
		},
		OnPatch: func(evt godex.StreamPatchEvent) {
			fmt.Printf("[patch %s] status=%s\n", evt.Patch.ID, evt.Patch.Status)
		},
		OnFileChange: func(evt godex.StreamFileChangeEvent) {
			fmt.Printf("  file %s (%s)\n", evt.Change.Path, evt.Change.Kind)
		},
		OnWebSearch: func(evt godex.StreamWebSearchEvent) {
			fmt.Printf("[web search] %s\n", evt.Search.Query)
		},
		OnThreadError: func(evt godex.ThreadErrorEvent) {
			log.Printf("[stream error] %s", evt.Message)
		},
	}

	result, err := thread.RunStreamed(context.Background(), "Summarize the latest SDK changes and list next steps.", &godex.TurnOptions{
		Callbacks: callbacks,
	})
	if err != nil {
		log.Fatalf("start streamed run: %v", err)
	}
	defer result.Close()

	for range result.Events() {
		// Drain events to honour backpressure; callbacks already handled rendering.
	}

	if err := result.Wait(); err != nil {
		log.Fatalf("stream failed: %v", err)
	}
}
