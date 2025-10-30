package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/activadee/godex"
)

func main() {
	client, err := godex.New(godex.CodexOptions{})
	if err != nil {
		log.Fatalf("create codex client: %v", err)
	}

	imagePath := filepath.Join("assets", "example.png")

	// Verify the image exists so the example fails fast if run from the wrong directory.
	if _, err := os.Stat(imagePath); err != nil {
		log.Fatalf("locate image %q: %v", imagePath, err)
	}

	thread := client.StartThread(godex.ThreadOptions{
		SkipGitRepoCheck: true,
		SandboxMode:      godex.SandboxModeDangerFullAccess,
	})

	segments := []godex.InputSegment{
		godex.TextSegment("Describe this image like you are writing alt text for documentation."),
		godex.LocalImageSegment(imagePath),
	}

	turn, err := thread.RunInputs(context.Background(), segments, nil)
	if err != nil {
		log.Fatalf("run thread: %v", err)
	}

	fmt.Println(turn.FinalResponse)
}
