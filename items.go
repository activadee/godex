package godex

// CommandExecutionStatus represents the lifecycle stage of a command started by the agent.
type CommandExecutionStatus string

const (
	CommandExecutionStatusInProgress CommandExecutionStatus = "in_progress"
	CommandExecutionStatusCompleted  CommandExecutionStatus = "completed"
	CommandExecutionStatusFailed     CommandExecutionStatus = "failed"
)

// CommandExecutionItem captures a command execution requested by the agent.
type CommandExecutionItem struct {
	ID               string                 `json:"id"`
	Type             string                 `json:"type"`
	Command          string                 `json:"command"`
	AggregatedOutput string                 `json:"aggregated_output"`
	ExitCode         *int                   `json:"exit_code,omitempty"`
	Status           CommandExecutionStatus `json:"status"`
}

// PatchChangeKind indicates how a file changed.
type PatchChangeKind string

const (
	PatchChangeKindAdd    PatchChangeKind = "add"
	PatchChangeKindDelete PatchChangeKind = "delete"
	PatchChangeKindUpdate PatchChangeKind = "update"
)

// FileUpdateChange represents a single file edit made by the agent.
type FileUpdateChange struct {
	Path string          `json:"path"`
	Kind PatchChangeKind `json:"kind"`
}

// PatchApplyStatus indicates whether the patch was applied successfully.
type PatchApplyStatus string

const (
	PatchApplyStatusCompleted PatchApplyStatus = "completed"
	PatchApplyStatusFailed    PatchApplyStatus = "failed"
)

// FileChangeItem aggregates the set of file updates that comprise a patch.
type FileChangeItem struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Changes []FileUpdateChange `json:"changes"`
	Status  PatchApplyStatus   `json:"status"`
}

// McpToolCallStatus describes the status of an MCP tool invocation.
type McpToolCallStatus string

const (
	McpToolCallStatusInProgress McpToolCallStatus = "in_progress"
	McpToolCallStatusCompleted  McpToolCallStatus = "completed"
	McpToolCallStatusFailed     McpToolCallStatus = "failed"
)

// McpToolCallItem represents activity relating to a single MCP tool call.
type McpToolCallItem struct {
	ID     string            `json:"id"`
	Type   string            `json:"type"`
	Server string            `json:"server"`
	Tool   string            `json:"tool"`
	Status McpToolCallStatus `json:"status"`
}

// AgentMessageItem contains the model's response payload (natural language or structured JSON).
type AgentMessageItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// ReasoningItem provides insight into the agent's intermediate reasoning.
type ReasoningItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// WebSearchItem denotes a web search performed by the agent.
type WebSearchItem struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Query string `json:"query"`
}

// ErrorItem captures non-fatal errors emitted by the agent.
type ErrorItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// TodoItem represents a single task within the agent's to-do list.
type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// TodoListItem tracks the set of tasks managed by the agent during a turn.
type TodoListItem struct {
	ID    string     `json:"id"`
	Type  string     `json:"type"`
	Items []TodoItem `json:"items"`
}

// ThreadItemType enumerates all valid thread item type strings.
type ThreadItemType string

// ThreadItemType constants.
const (
	ThreadItemTypeAgentMessage     ThreadItemType = "agent_message"
	ThreadItemTypeReasoning        ThreadItemType = "reasoning"
	ThreadItemTypeCommandExecution ThreadItemType = "command_execution"
	ThreadItemTypeFileChange       ThreadItemType = "file_change"
	ThreadItemTypeMcpToolCall      ThreadItemType = "mcp_tool_call"
	ThreadItemTypeWebSearch        ThreadItemType = "web_search"
	ThreadItemTypeTodoList         ThreadItemType = "todo_list"
	ThreadItemTypeError            ThreadItemType = "error"
)

// ThreadItem is the polymorphic representation of all items that can appear on a thread.
type ThreadItem interface {
	threadItem()
	itemType() ThreadItemType
}

func (AgentMessageItem) threadItem()     {}
func (ReasoningItem) threadItem()        {}
func (CommandExecutionItem) threadItem() {}
func (FileChangeItem) threadItem()       {}
func (McpToolCallItem) threadItem()      {}
func (WebSearchItem) threadItem()        {}
func (TodoListItem) threadItem()         {}
func (ErrorItem) threadItem()            {}

func (AgentMessageItem) itemType() ThreadItemType     { return ThreadItemTypeAgentMessage }
func (ReasoningItem) itemType() ThreadItemType        { return ThreadItemTypeReasoning }
func (CommandExecutionItem) itemType() ThreadItemType { return ThreadItemTypeCommandExecution }
func (FileChangeItem) itemType() ThreadItemType       { return ThreadItemTypeFileChange }
func (McpToolCallItem) itemType() ThreadItemType      { return ThreadItemTypeMcpToolCall }
func (WebSearchItem) itemType() ThreadItemType        { return ThreadItemTypeWebSearch }
func (TodoListItem) itemType() ThreadItemType         { return ThreadItemTypeTodoList }
func (ErrorItem) itemType() ThreadItemType            { return ThreadItemTypeError }
