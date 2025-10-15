package main

import (
	"context"
	"fmt"
	"log"

	"github.com/activadee/godex"
)

func main() {
	// CodexOptions allows overriding the binary path, base URL, or API key when needed.
	client, err := godex.New(godex.CodexOptions{})
	if err != nil {
		log.Fatalf("create codex client: %v", err)
	}

	thread := client.StartThread(godex.ThreadOptions{
		SkipGitRepoCheck: true,
		SandboxMode:      godex.SandboxModeDangerFullAccess,
	})

	turn, err := thread.Run(context.Background(), "Say hello from Codex.", nil)
	if err != nil {
		log.Fatalf("run thread: %v", err)
	}

	fmt.Println(turn.FinalResponse)
}
