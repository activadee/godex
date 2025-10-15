package godex

import (
	"context"
	"fmt"
	"sync"

	"github.com/activadee/godex/internal/codexexec"
)

// Turn represents a fully completed turn from the Codex agent.
type Turn struct {
	Items         []ThreadItem
	FinalResponse string
	Usage         *Usage
}

// RunResult is an alias for Turn to mirror the TypeScript SDK naming.
type RunResult = Turn

// RunStreamedResult is returned by Thread.RunStreamed and exposes the event stream.
type RunStreamedResult struct {
	stream *Stream
}

// Events returns the channel that yields events sequentially as they arrive.
func (r RunStreamedResult) Events() <-chan ThreadEvent {
	if r.stream == nil {
		ch := make(chan ThreadEvent)
		close(ch)
		return ch
	}
	return r.stream.Events()
}

// Wait blocks until the stream finishes and returns the terminal error, if any.
func (r RunStreamedResult) Wait() error {
	if r.stream == nil {
		return nil
	}
	return r.stream.Wait()
}

// Close cancels the stream context and waits for shutdown.
func (r RunStreamedResult) Close() error {
	if r.stream == nil {
		return nil
	}
	return r.stream.Close()
}

// Thread encapsulates a conversation with the Codex agent. It is safe to reuse a Thread
// across sequential turns, but concurrent Run/RunStreamed calls on the same Thread are not supported.
type Thread struct {
	exec          *codexexec.Runner
	options       CodexOptions
	threadOptions ThreadOptions

	mu sync.RWMutex
	id string
}

func newThread(exec *codexexec.Runner, options CodexOptions, threadOptions ThreadOptions, id string) *Thread {
	return &Thread{
		exec:          exec,
		options:       options,
		threadOptions: threadOptions,
		id:            id,
	}
}

// ID returns the identifier of the thread. For new threads this becomes available after
// the first `thread.started` event is received.
func (t *Thread) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

// RunStreamed submits the provided input to the agent and streams events as they occur.
func (t *Thread) RunStreamed(ctx context.Context, input string, turnOptions *TurnOptions) (RunStreamedResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var turnOpts TurnOptions
	if turnOptions != nil {
		turnOpts = *turnOptions
	}

	schemaPath, cleanup, err := createOutputSchemaFile(turnOpts.OutputSchema)
	if err != nil {
		return RunStreamedResult{}, err
	}

	ctx, cancel := context.WithCancel(ctx)
	events := make(chan ThreadEvent)
	stream := newStream(events, cancel)

	currentThreadID := t.ID()

	go func() {
		defer close(events)
		defer stream.finish()
		defer cleanup()
		args := codexexec.Args{
			Input:            input,
			BaseURL:          t.options.BaseURL,
			APIKey:           t.options.APIKey,
			ThreadID:         currentThreadID,
			Model:            t.threadOptions.Model,
			SandboxMode:      string(t.threadOptions.SandboxMode),
			WorkingDirectory: t.threadOptions.WorkingDirectory,
			SkipGitRepoCheck: t.threadOptions.SkipGitRepoCheck,
			OutputSchemaPath: schemaPath,
		}

		err := t.exec.Run(ctx, args, func(line []byte) error {
			event, decodeErr := decodeThreadEvent(line)
			if decodeErr != nil {
				return fmt.Errorf("parse event: %w", decodeErr)
			}

			if started, ok := event.(ThreadStartedEvent); ok {
				t.setID(started.ThreadID)
			}

			select {
			case events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		stream.setErr(err)
	}()

	return RunStreamedResult{stream: stream}, nil
}

// Run submits the input to the agent and waits for the turn to finish, returning the final response.
func (t *Thread) Run(ctx context.Context, input string, turnOptions *TurnOptions) (RunResult, error) {
	result, err := t.RunStreamed(ctx, input, turnOptions)
	if err != nil {
		return RunResult{}, err
	}
	defer result.Close()

	var (
		items        []ThreadItem
		finalMessage string
		varUsage     *Usage
		turnFailure  *ThreadError
	)

	for event := range result.Events() {
		switch e := event.(type) {
		case ItemCompletedEvent:
			items = append(items, e.Item)
			if message, ok := e.Item.(AgentMessageItem); ok {
				finalMessage = message.Text
			}
		case TurnCompletedEvent:
			usageCopy := e.Usage
			varUsage = &usageCopy
		case TurnFailedEvent:
			turnFailure = &e.Error
		case ThreadErrorEvent:
			return RunResult{}, fmt.Errorf("thread error: %s", e.Message)
		}

		if turnFailure != nil {
			break
		}
	}

	if err := result.Wait(); err != nil {
		return RunResult{}, err
	}

	if turnFailure != nil {
		return RunResult{}, fmt.Errorf(turnFailure.Message)
	}

	return RunResult{
		Items:         items,
		FinalResponse: finalMessage,
		Usage:         varUsage,
	}, nil
}

func (t *Thread) setID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.id = id
}
