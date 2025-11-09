# godex – Go SDK for Codex

`godex` is an idiomatic Go wrapper around the Codex CLI. It mirrors the ergonomics of the official TypeScript SDK so that you can integrate Codex agents into Go applications with just a few calls.

## Installation

```bash
go get github.com/activadee/godex
```

`godex` automatically downloads the Codex CLI into your user cache the first time it is needed. The cached build is keyed by platform and release tag so upgrades are seamless. Advanced users can override the cache directory or release tag via `CodexOptions.CLICacheDir` / `CodexOptions.CLIReleaseTag` (or the `GODEX_CLI_CACHE` / `GODEX_CLI_RELEASE_TAG` environment variables) and enforce SHA-256 verification with `CodexOptions.CLIChecksum` or `GODEX_CLI_CHECKSUM`. If you prefer to use a self-managed binary, set `CodexOptions.CodexPathOverride` or ensure the CLI is already available on your `PATH` (e.g. `which codex`). Authentication is handled entirely by the CLI; reuse whichever credentials you already configured (environment variables, `codex auth login`, etc.) or set `CodexOptions.APIKey` to override the API key programmatically:

```bash
export CODEX_API_KEY=sk-...
# or configure the CLI interactively
codex auth login
```

Override service endpoints (for self-hosted deployments) with `CodexOptions.BaseURL`.

### CLI bootstrap controls

`CodexOptions` exposes the same knobs that previously required environment variables when
you need deterministic CLI bootstrapping:

- `CLICacheDir` overrides where downloaded binaries are stored. It takes precedence over
  `GODEX_CLI_CACHE` and falls back to the user cache or `os.TempDir()`.
- `CLIReleaseTag` pins the release asset fetched from `github.com/openai/codex`. It overrides
  `GODEX_CLI_RELEASE_TAG` and defaults to the SDK's bundled tag.
- `CLIChecksum` enforces integrity by verifying the SHA-256 checksum of the extracted binary.
  Supply the expected digest (hex encoded) from the official release notes or your
  distribution channel. The environment variable equivalent is `GODEX_CLI_CHECKSUM`.

```go
import (
	"os"
	"path/filepath"

	"github.com/activadee/godex"
)

client, err := godex.New(godex.CodexOptions{
	CLICacheDir:   filepath.Join(os.TempDir(), "codex-cache"),
	CLIReleaseTag: "rust-v0.55.0",
	CLIChecksum:   "f8a1...",
})
```

When a checksum is configured, `godex` verifies both cached binaries and freshly downloaded
ones, forcing a re-download or returning an error if the digest does not match. This allows
you to gate Codex upgrades on an allowlisted fingerprint without writing custom bootstrap
code. The checksum is calculated over the extracted `codex` executable for the detected
platform/architecture.

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

### Streaming callbacks

Set `TurnOptions.Callbacks` to receive typed updates without writing a `switch` over
`ThreadEvent`. The SDK invokes `OnEvent` first, followed by any matching typed callbacks,
and finally forwards the raw event through `Events()`. Callbacks run on the streaming goroutine,
so long-running work should be offloaded to avoid stalling the stream. You must continue
draining the `Events()` channel (an empty `for range` loop works) to honour the CLI's
backpressure expectations.

```go
callbacks := &godex.StreamCallbacks{
	OnMessage: func(evt godex.StreamMessageEvent) {
		if evt.Stage == godex.StreamItemStageCompleted {
			log.Printf("assistant: %s", evt.Message.Text)
		}
	},
	OnCommand: func(evt godex.StreamCommandEvent) {
		log.Printf("command %s: %s", evt.Command.Status, evt.Command.Command)
	},
	OnPatch: func(evt godex.StreamPatchEvent) {
		log.Printf("patch %s: %s", evt.Patch.ID, evt.Patch.Status)
	},
	OnFileChange: func(evt godex.StreamFileChangeEvent) {
		log.Printf("  file %s (%s)", evt.Change.Path, evt.Change.Kind)
	},
}

result, err := thread.RunStreamed(ctx, "Summarize the latest changes.", &godex.TurnOptions{
	Callbacks: callbacks,
})
if err != nil {
	log.Fatal(err)
}
defer result.Close()

for range result.Events() {
	// Drain events; callbacks handled the typed work already.
}

if err := result.Wait(); err != nil {
	log.Fatal(err)
}
```

See `examples/streaming_callbacks` for a complete runnable sample.

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

### Typed helpers

Generate and decode structured JSON into Go types with `RunJSON` / `RunStreamedJSON`. Provide
your own schema or allow the helpers to infer one from `T`:

```go
type Update struct {
 Headline string `json:"headline"`
 NextStep string `json:"next_step"`
}

result, err := godex.RunJSON[Update](ctx, thread, "Provide a concise update.", nil)
if err != nil {
 log.Fatal(err)
}
log.Printf("update: %+v", result)
```

## Multi-part input and images

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

For remote assets or in-memory data, reach for the convenience constructors:

```go
segment, err := godex.URLImageSegment(ctx, "https://example.com/image.png")
if err != nil {
 log.Fatal(err)
}

rawBytes := loadThumbnailBytes() // your own code that returns []byte

bytesSegment, err := godex.BytesImageSegment("thumbnail", rawBytes)
if err != nil {
 log.Fatal(err)
}

turn, err := thread.RunInputs(ctx, []godex.InputSegment{
 godex.TextSegment("Describe both images."),
 segment,
 bytesSegment,
}, nil)
```

`URLImageSegment` downloads the image to a temp file, verifies that the server returned an
`image/*` content type, and schedules the file for cleanup once the run completes. Use
`BytesImageSegment` when you already have the image bytes; it writes them to a temporary file
with a suitable extension and cleans the file up automatically.

## Examples

- `examples/basic`: single-turn conversation (`go run ./examples/basic`)
- `examples/streaming`: step-by-step event streaming demo (`go run ./examples/streaming`)
- `examples/schema`: structured JSON output with schema validation (`go run ./examples/schema`)
- `examples/structured_output`: typed structured output helpers (`go run ./examples/structured_output`)
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

## Selecting a profile programmatically

Set CLI configuration overrides on `CodexOptions.ConfigOverrides`. Any key named `profile` is forwarded as `--profile`, while the rest become `-c key=value` pairs:

```go
client, err := godex.New(godex.CodexOptions{
 ConfigOverrides: map[string]any{
  "profile":        "production",
  "feature.toggle": true,
 },
})
if err != nil {
 log.Fatal(err)
}
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
