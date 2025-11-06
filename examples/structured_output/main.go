package main

import (
	"context"
	"fmt"
	"log"

	"github.com/activadee/godex"
)

type projectUpdate struct {
	Headline string `json:"headline" jsonschema:"description=Short summary of the update"`
	NextStep string `json:"next_step" jsonschema:"description=Concrete follow-up action"`
}

func main() {
	client, err := godex.New(godex.CodexOptions{})
	if err != nil {
		log.Fatalf("create codex client: %v", err)
	}

	thread := client.StartThread(godex.ThreadOptions{
		Model: "gpt-5",
	})

	update, err := godex.RunJSON[projectUpdate](context.Background(), thread, "Provide a concise project update and a suggested next step.", nil)
	if err != nil {
		log.Fatalf("run structured turn: %v", err)
	}

	fmt.Printf("Headline: %s\nNext step: %s\n", update.Headline, update.NextStep)

	streamed, err := godex.RunStreamedJSON[projectUpdate](context.Background(), thread, "Give another update and next step, streaming partial results.", nil)
	if err != nil {
		log.Fatalf("start streamed structured turn: %v", err)
	}
	defer streamed.Close()

	for update := range streamed.Updates() {
		fmt.Printf("[structured update] final=%t headline=%q next_step=%q\n", update.Final, update.Value.Headline, update.Value.NextStep)
	}

	if err := streamed.Wait(); err != nil {
		log.Fatalf("streamed structured turn failed: %v", err)
	}
}
