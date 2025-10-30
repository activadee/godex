# godex – Go SDK for Codex

`godex` is an idiomatic Go wrapper around the Codex CLI. It mirrors the ergonomics of the official TypeScript SDK so that you can integrate Codex agents into Go applications with just a few calls.

## Installation

```bash
go get github.com/activadee/godex
```

`godex` automatically downloads the Codex CLI into your user cache the first time it is needed. The cached build is keyed by platform and release tag so upgrades are seamless. Advanced users can override the cache directory or release tag via the `GODEX_CLI_CACHE` and `GODEX_CLI_RELEASE_TAG` environment variables. If you prefer to use a self-managed binary, set `CodexOptions.CodexPathOverride` or ensure the CLI is already available on your `PATH` (e.g. `which codex`). Authentication is handled entirely by the CLI; reuse whichever credentials you already configured (environment variables, `codex auth login`, etc.) or set `CodexOptions.APIKey` to override the API key programmatically:

```bash
export CODEX_API_KEY=sk-...
# or configure the CLI interactively
codex auth login
```

Override service endpoints (for self-hosted deployments) with `CodexOptions.BaseURL`.

## Quick start

```go
package main

import (
 "context"
 "fmt"

 "github.com/activadee/godex"
)

func main() {
 c, err := godex.New(godex.CodexOptions{})
 if err != nil {
  panic(err)
 }

 thread := c.StartThread(godex.ThreadOptions{
  Model: "gpt-5",
 })

 turn, err := thread.Run(context.Background(), "List three quick wins to speed up CI?", nil)
 if err != nil {
  panic(err)
 }

 fmt.Println("Assistant:", turn.FinalResponse)
}
```

## Streaming events

```go
ctx := context.Background()
result, err := thread.RunStreamed(ctx, "Summarize the latest changes.", nil)
if err != nil {
 log.Fatal(err)
}
defer result.Close()

for event := range result.Events() {
 switch e := event.(type) {
 case godex.ItemStartedEvent:
  log.Printf("item started: %T", e.Item)
 case godex.ItemCompletedEvent:
  log.Printf("item completed: %T", e.Item)
 case godex.TurnCompletedEvent:
  log.Printf("usage: %+v", e.Usage)
 }
}

if err := result.Wait(); err != nil {
 log.Fatal(err)
}
```

## Structured output

Pass a JSON schema in `TurnOptions.OutputSchema` and the SDK writes a temporary file for the CLI:

```go
schema := map[string]any{
 "type": "object",
 "properties": map[string]any{
  "summary": map[string]any{"type": "string"},
 },
 "required": []string{"summary"},
}

turn, err := thread.Run(ctx, "Write a one sentence update.", &godex.TurnOptions{
 OutputSchema: schema,
})
```

## Multi-part input and local images

Mix text segments and local image paths by using `RunInputs` / `RunStreamedInputs` with
`InputSegment` helpers. Text segments are joined with blank lines and each image path is
forwarded to the CLI via `--image`:

```go
segments := []godex.InputSegment{
 godex.TextSegment("Describe the image differences"),
 godex.LocalImageSegment("/tmp/baseline.png"),
 godex.LocalImageSegment("/tmp/current.png"),
}

turn, err := thread.RunInputs(ctx, segments, nil)
if err != nil {
 log.Fatal(err)
}

fmt.Println("Assistant:", turn.FinalResponse)
```

## Examples

- `examples/basic`: single-turn conversation (`go run ./examples/basic`)
- `examples/streaming`: step-by-step event streaming demo (`go run ./examples/streaming`)
- `examples/schema`: structured JSON output with schema validation (`go run ./examples/schema`)
- `examples/images`: multi-part prompt mixing text and a local image (`go run ./examples/images`)

## Thread persistence

Threads expose their ID once the `thread.started` event arrives. Store it and recreate a thread later:

```go
savedID := thread.ID()
resumed := c.ResumeThread(savedID, godex.ThreadOptions{})
```

## Sandbox settings

Configure the CLI sandbox, working directory, and git guardrails via `ThreadOptions`:

```go
thread := c.StartThread(godex.ThreadOptions{
 SandboxMode:      godex.SandboxModeWorkspaceWrite,
 WorkingDirectory: "/tmp/workspace",
 SkipGitRepoCheck: true,
})
```

## Development

Run the tests locally:

```bash
go test ./...
```

Because the CLI is not bundled with the repository, integration tests that spawn Codex are not included. Use `CodexOptions.CodexPathOverride` to point at a custom binary when exercising end-to-end flows.

## Error handling

`Thread.Run` and `Thread.RunStreamed` surface failures in a few ways:

- Turn-level errors (`turn.failed` events) return a Go `error` whose message mirrors the CLI output.
- Stream-level errors (`error` events) abort the stream with a `*godex.ThreadStreamError`, exposing the reported message and allowing `errors.As` checks.
- Process failures (non-zero CLI exit) propagate the exit code and stderr via `Runner.Run`.

Always check the returned error when the agent turn completes.
