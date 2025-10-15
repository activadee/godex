package godex

import (
	"encoding/json"
	"errors"
	"fmt"
)

// decodeThreadEvent converts a JSON line produced by the Codex CLI into a strongly typed event.
func decodeThreadEvent(data []byte) (ThreadEvent, error) {
	var base struct {
		Type ThreadEventType `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("decode event envelope: %w", err)
	}

	switch base.Type {
	case ThreadEventTypeThreadStarted:
		var event ThreadStartedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode thread.started event: %w", err)
		}
		return event, nil
	case ThreadEventTypeTurnStarted:
		var event TurnStartedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode turn.started event: %w", err)
		}
		return event, nil
	case ThreadEventTypeTurnCompleted:
		var event TurnCompletedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode turn.completed event: %w", err)
		}
		return event, nil
	case ThreadEventTypeTurnFailed:
		var event TurnFailedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode turn.failed event: %w", err)
		}
		return event, nil
	case ThreadEventTypeItemStarted:
		return decodeItemEvent(data, ThreadEventTypeItemStarted)
	case ThreadEventTypeItemUpdated:
		return decodeItemEvent(data, ThreadEventTypeItemUpdated)
	case ThreadEventTypeItemCompleted:
		return decodeItemEvent(data, ThreadEventTypeItemCompleted)
	case ThreadEventTypeError:
		var event ThreadErrorEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, fmt.Errorf("decode error event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", base.Type)
	}
}

func decodeItemEvent(data []byte, eventType ThreadEventType) (ThreadEvent, error) {
	var envelope struct {
		Type ThreadEventType `json:"type"`
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s envelope: %w", eventType, err)
	}

	item, err := decodeThreadItem(envelope.Item)
	if err != nil {
		return nil, fmt.Errorf("decode %s item: %w", eventType, err)
	}

	switch eventType {
	case ThreadEventTypeItemStarted:
		return ItemStartedEvent{Type: eventType, Item: item}, nil
	case ThreadEventTypeItemUpdated:
		return ItemUpdatedEvent{Type: eventType, Item: item}, nil
	case ThreadEventTypeItemCompleted:
		return ItemCompletedEvent{Type: eventType, Item: item}, nil
	default:
		return nil, errors.New("invalid item event type")
	}
}

// decodeThreadItem converts raw JSON into a specific ThreadItem implementation.
func decodeThreadItem(data []byte) (ThreadItem, error) {
	var base struct {
		Type ThreadItemType `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("decode item envelope: %w", err)
	}

	switch base.Type {
	case ThreadItemTypeAgentMessage:
		var item AgentMessageItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode agent message item: %w", err)
		}
		return item, nil
	case ThreadItemTypeReasoning:
		var item ReasoningItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode reasoning item: %w", err)
		}
		return item, nil
	case ThreadItemTypeCommandExecution:
		var item CommandExecutionItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode command execution item: %w", err)
		}
		return item, nil
	case ThreadItemTypeFileChange:
		var item FileChangeItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode file change item: %w", err)
		}
		return item, nil
	case ThreadItemTypeMcpToolCall:
		var item McpToolCallItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode MCP tool call item: %w", err)
		}
		return item, nil
	case ThreadItemTypeWebSearch:
		var item WebSearchItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode web search item: %w", err)
		}
		return item, nil
	case ThreadItemTypeTodoList:
		var item TodoListItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode todo list item: %w", err)
		}
		return item, nil
	case ThreadItemTypeError:
		var item ErrorItem
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode error item: %w", err)
		}
		return item, nil
	default:
		return nil, fmt.Errorf("unknown item type %q", base.Type)
	}
}
