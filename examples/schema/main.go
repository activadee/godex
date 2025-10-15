package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/activadee/godex"
)

type Update struct {
	Headline string `json:"headline"`
	NextStep string `json:"next_step"`
}

func main() {
	client, err := godex.New(godex.CodexOptions{})
	if err != nil {
		log.Fatalf("create codex client: %v", err)
	}

	thread := client.StartThread(godex.ThreadOptions{
		Model: "gpt-5",
	})

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": map[string]any{
				"type":        "string",
				"description": "Short summary of the update",
			},
			"next_step": map[string]any{
				"type":        "string",
				"description": "Concrete follow-up action",
			},
		},
		"required": []string{"headline", "next_step"},
	}

	turn, err := thread.Run(context.Background(), "Provide a concise project update and a suggested next step.", &godex.TurnOptions{
		OutputSchema: schema,
	})
	if err != nil {
		log.Fatalf("run thread: %v", err)
	}

	var update Update
	if err := json.Unmarshal([]byte(turn.FinalResponse), &update); err != nil {
		log.Fatalf("decode structured response: %v\nraw: %s", err, turn.FinalResponse)
	}

	fmt.Printf("Headline: %s\nNext step: %s\n", update.Headline, update.NextStep)
}
