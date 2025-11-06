package godex

// StreamItemStage indicates which phase of the lifecycle produced a callback.
type StreamItemStage string

const (
	StreamItemStageStarted   StreamItemStage = "started"
	StreamItemStageUpdated   StreamItemStage = "updated"
	StreamItemStageCompleted StreamItemStage = "completed"
)

// StreamMessageEvent describes a callback payload for agent message items.
type StreamMessageEvent struct {
	Stage   StreamItemStage
	Message AgentMessageItem
}

// StreamReasoningEvent describes a callback payload for reasoning items.
type StreamReasoningEvent struct {
	Stage     StreamItemStage
	Reasoning ReasoningItem
}

// StreamCommandEvent describes a callback payload for command execution items.
type StreamCommandEvent struct {
	Stage   StreamItemStage
	Command CommandExecutionItem
}

// StreamPatchEvent describes a callback payload for patch/file change items.
type StreamPatchEvent struct {
	Stage StreamItemStage
	Patch FileChangeItem
}

// StreamFileChangeEvent describes a callback payload for each file updated within a patch.
type StreamFileChangeEvent struct {
	Stage  StreamItemStage
	Patch  FileChangeItem
	Change FileUpdateChange
}

// StreamWebSearchEvent describes a callback payload for web search items.
type StreamWebSearchEvent struct {
	Stage  StreamItemStage
	Search WebSearchItem
}

// StreamToolCallEvent describes a callback payload for MCP tool call items.
type StreamToolCallEvent struct {
	Stage    StreamItemStage
	ToolCall McpToolCallItem
}

// StreamTodoListEvent describes a callback payload for todo list items.
type StreamTodoListEvent struct {
	Stage StreamItemStage
	List  TodoListItem
}

// StreamErrorItemEvent describes a callback payload for non-fatal error items.
type StreamErrorItemEvent struct {
	Stage StreamItemStage
	Error ErrorItem
}

// StreamCallbacks enumerates optional hooks invoked when streaming events are delivered.
type StreamCallbacks struct {
	// OnEvent fires for every event before any type-specific callback.
	OnEvent func(ThreadEvent)

	OnThreadStarted func(ThreadStartedEvent)
	OnTurnStarted   func(TurnStartedEvent)
	OnTurnCompleted func(TurnCompletedEvent)
	OnTurnFailed    func(TurnFailedEvent)
	OnThreadError   func(ThreadErrorEvent)

	OnMessage    func(StreamMessageEvent)
	OnReasoning  func(StreamReasoningEvent)
	OnCommand    func(StreamCommandEvent)
	OnPatch      func(StreamPatchEvent)
	OnFileChange func(StreamFileChangeEvent)
	OnWebSearch  func(StreamWebSearchEvent)
	OnToolCall   func(StreamToolCallEvent)
	OnTodoList   func(StreamTodoListEvent)
	OnErrorItem  func(StreamErrorItemEvent)
}

func (c *StreamCallbacks) handle(event ThreadEvent) {
	if c == nil {
		return
	}

	if c.OnEvent != nil {
		c.OnEvent(event)
	}

	switch e := event.(type) {
	case ThreadStartedEvent:
		if c.OnThreadStarted != nil {
			c.OnThreadStarted(e)
		}
	case TurnStartedEvent:
		if c.OnTurnStarted != nil {
			c.OnTurnStarted(e)
		}
	case TurnCompletedEvent:
		if c.OnTurnCompleted != nil {
			c.OnTurnCompleted(e)
		}
	case TurnFailedEvent:
		if c.OnTurnFailed != nil {
			c.OnTurnFailed(e)
		}
	case ThreadErrorEvent:
		if c.OnThreadError != nil {
			c.OnThreadError(e)
		}
	case ItemStartedEvent:
		c.handleItem(StreamItemStageStarted, e.Item)
	case ItemUpdatedEvent:
		c.handleItem(StreamItemStageUpdated, e.Item)
	case ItemCompletedEvent:
		c.handleItem(StreamItemStageCompleted, e.Item)
	}
}

func (c *StreamCallbacks) handleItem(stage StreamItemStage, item ThreadItem) {
	if c == nil || item == nil {
		return
	}

	switch v := item.(type) {
	case AgentMessageItem:
		if c.OnMessage != nil {
			c.OnMessage(StreamMessageEvent{Stage: stage, Message: v})
		}
	case ReasoningItem:
		if c.OnReasoning != nil {
			c.OnReasoning(StreamReasoningEvent{Stage: stage, Reasoning: v})
		}
	case CommandExecutionItem:
		if c.OnCommand != nil {
			c.OnCommand(StreamCommandEvent{Stage: stage, Command: v})
		}
	case FileChangeItem:
		if c.OnPatch != nil {
			c.OnPatch(StreamPatchEvent{Stage: stage, Patch: v})
		}
		if c.OnFileChange != nil {
			for _, change := range v.Changes {
				c.OnFileChange(StreamFileChangeEvent{
					Stage:  stage,
					Patch:  v,
					Change: change,
				})
			}
		}
	case McpToolCallItem:
		if c.OnToolCall != nil {
			c.OnToolCall(StreamToolCallEvent{Stage: stage, ToolCall: v})
		}
	case WebSearchItem:
		if c.OnWebSearch != nil {
			c.OnWebSearch(StreamWebSearchEvent{Stage: stage, Search: v})
		}
	case TodoListItem:
		if c.OnTodoList != nil {
			c.OnTodoList(StreamTodoListEvent{Stage: stage, List: v})
		}
	case ErrorItem:
		if c.OnErrorItem != nil {
			c.OnErrorItem(StreamErrorItemEvent{Stage: stage, Error: v})
		}
	}
}
