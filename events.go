package godex

// Usage captures token consumption metrics for a completed turn.
type Usage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

// ThreadError represents a fatal error emitted for the turn.
type ThreadError struct {
	Message string `json:"message"`
}

// ThreadEventType enumerates the JSON event types streamed by the Codex CLI.
type ThreadEventType string

const (
	ThreadEventTypeThreadStarted ThreadEventType = "thread.started"
	ThreadEventTypeTurnStarted   ThreadEventType = "turn.started"
	ThreadEventTypeTurnCompleted ThreadEventType = "turn.completed"
	ThreadEventTypeTurnFailed    ThreadEventType = "turn.failed"
	ThreadEventTypeItemStarted   ThreadEventType = "item.started"
	ThreadEventTypeItemUpdated   ThreadEventType = "item.updated"
	ThreadEventTypeItemCompleted ThreadEventType = "item.completed"
	ThreadEventTypeError         ThreadEventType = "error"
)

// ThreadEvent is the interface implemented by all event variants returned by the CLI.
type ThreadEvent interface {
	threadEvent()
	EventType() ThreadEventType
}

// ThreadStartedEvent is emitted when a new thread is created.
type ThreadStartedEvent struct {
	Type     ThreadEventType `json:"type"`
	ThreadID string          `json:"thread_id"`
}

func (ThreadStartedEvent) threadEvent()                 {}
func (e ThreadStartedEvent) EventType() ThreadEventType { return e.Type }

// TurnStartedEvent marks the beginning of a new turn.
type TurnStartedEvent struct {
	Type ThreadEventType `json:"type"`
}

func (TurnStartedEvent) threadEvent()                 {}
func (e TurnStartedEvent) EventType() ThreadEventType { return e.Type }

// TurnCompletedEvent indicates a successful completion of a turn.
type TurnCompletedEvent struct {
	Type  ThreadEventType `json:"type"`
	Usage Usage           `json:"usage"`
}

func (TurnCompletedEvent) threadEvent()                 {}
func (e TurnCompletedEvent) EventType() ThreadEventType { return e.Type }

// TurnFailedEvent signals that a turn ended due to a fatal error.
type TurnFailedEvent struct {
	Type  ThreadEventType `json:"type"`
	Error ThreadError     `json:"error"`
}

func (TurnFailedEvent) threadEvent()                 {}
func (e TurnFailedEvent) EventType() ThreadEventType { return e.Type }

// ItemStartedEvent emits when a thread item is created.
type ItemStartedEvent struct {
	Type ThreadEventType `json:"type"`
	Item ThreadItem      `json:"item"`
}

func (ItemStartedEvent) threadEvent()                 {}
func (e ItemStartedEvent) EventType() ThreadEventType { return e.Type }

// ItemUpdatedEvent emits as an item transitions between intermediate states.
type ItemUpdatedEvent struct {
	Type ThreadEventType `json:"type"`
	Item ThreadItem      `json:"item"`
}

func (ItemUpdatedEvent) threadEvent()                 {}
func (e ItemUpdatedEvent) EventType() ThreadEventType { return e.Type }

// ItemCompletedEvent signals an item reaching a terminal state.
type ItemCompletedEvent struct {
	Type ThreadEventType `json:"type"`
	Item ThreadItem      `json:"item"`
}

func (ItemCompletedEvent) threadEvent()                 {}
func (e ItemCompletedEvent) EventType() ThreadEventType { return e.Type }

// ThreadErrorEvent is emitted when the stream itself experiences an unrecoverable error.
type ThreadErrorEvent struct {
	Type    ThreadEventType `json:"type"`
	Message string          `json:"message"`
}

func (ThreadErrorEvent) threadEvent()                 {}
func (e ThreadErrorEvent) EventType() ThreadEventType { return e.Type }
