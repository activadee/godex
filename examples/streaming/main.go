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

	result, err := thread.RunStreamed(context.Background(), "Summarize the current repository structure.", nil)
	if err != nil {
		log.Fatalf("start streamed run: %v", err)
	}
	defer result.Close()

	for event := range result.Events() {
		switch e := event.(type) {
		case godex.ItemStartedEvent:
			fmt.Printf("[item started] %T\n", e.Item)
		case godex.ItemUpdatedEvent:
			fmt.Printf("[item updated] %T\n", e.Item)
		case godex.ItemCompletedEvent:
			fmt.Printf("[item completed] %T\n", e.Item)
			if msg, ok := e.Item.(godex.AgentMessageItem); ok {
				fmt.Printf("assistant: %s\n", msg.Text)
			}
		case godex.TurnCompletedEvent:
			fmt.Printf("[turn completed] usage=%+v\n", e.Usage)
		case godex.TurnFailedEvent:
			log.Fatalf("turn failed: %s", e.Error.Message)
		case godex.ThreadErrorEvent:
			log.Fatalf("stream error: %s", e.Message)
		}
	}

	if err := result.Wait(); err != nil {
		log.Fatalf("stream error: %v", err)
	}
}
