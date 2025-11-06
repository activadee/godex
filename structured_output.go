package godex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/invopop/jsonschema"
)

var (
	// ErrNoStructuredOutput indicates that the turn completed without returning a structured
	// response that could be decoded into the requested type.
	ErrNoStructuredOutput = errors.New("structured output not returned")
)

const runStreamedJSONEventBuffer = 16

// RunJSONOptions configure a typed JSON turn.
type RunJSONOptions[T any] struct {
	// TurnOptions forwards additional options for the turn. When nil a zero TurnOptions
	// value is used.
	TurnOptions *TurnOptions
	// Schema provides an explicit JSON schema for the structured output. When nil the
	// helper attempts schema inference unless DisableSchemaInference is true.
	Schema any
	// DisableSchemaInference prevents automatic schema inference from T when Schema is nil.
	DisableSchemaInference bool
}

// SchemaViolationError indicates that the structured output failed schema validation.
type SchemaViolationError struct {
	Message string
}

// Error implements the error interface.
func (e *SchemaViolationError) Error() string {
	if e == nil || e.Message == "" {
		return "structured output schema violation"
	}
	return e.Message
}

// RunJSON executes a turn expecting a structured JSON response that can be decoded into T.
func RunJSON[T any](ctx context.Context, thread *Thread, input string, options *RunJSONOptions[T]) (T, error) {
	var zero T

	if thread == nil {
		return zero, errors.New("RunJSON requires a non-nil thread")
	}

	config, err := prepareRunJSONOptions[T](options)
	if err != nil {
		return zero, err
	}

	result, err := thread.run(ctx, input, nil, &config.turnOptions)
	if err != nil {
		if schemaErr, ok := classifyStructuredOutputError(err, config.expectSchemaError); ok {
			return zero, schemaErr
		}
		return zero, err
	}

	var value T
	if err := json.Unmarshal([]byte(result.FinalResponse), &value); err != nil {
		return zero, fmt.Errorf("decode structured output: %w", err)
	}
	return value, nil
}

// RunStreamedJSONUpdate captures a typed snapshot of the structured output as the turn progresses.
type RunStreamedJSONUpdate[T any] struct {
	Value T
	Raw   string
	Final bool
}

// RunStreamedJSONResult exposes the streaming lifecycle for a typed structured output turn.
type RunStreamedJSONResult[T any] struct {
	stream  *Stream
	events  <-chan ThreadEvent
	updates <-chan RunStreamedJSONUpdate[T]
	err     *sharedError
}

// Events returns the stream of raw thread events produced by the turn.
func (r RunStreamedJSONResult[T]) Events() <-chan ThreadEvent {
	return r.events
}

// Updates yields typed structured output snapshots. The channel closes once the turn finishes.
func (r RunStreamedJSONResult[T]) Updates() <-chan RunStreamedJSONUpdate[T] {
	return r.updates
}

// Wait blocks until the turn finishes and returns the terminal error, if any.
func (r RunStreamedJSONResult[T]) Wait() error {
	if r.stream == nil {
		return nil
	}
	if err := r.stream.Wait(); err != nil {
		return err
	}
	return r.err.get()
}

// Close cancels the turn and waits for shutdown.
func (r RunStreamedJSONResult[T]) Close() error {
	if r.stream == nil {
		return nil
	}
	if err := r.stream.Close(); err != nil {
		return err
	}
	return r.err.get()
}

// RunStreamedJSON executes a turn expecting structured JSON output and streams raw events
// alongside typed snapshots decoded into T.
func RunStreamedJSON[T any](ctx context.Context, thread *Thread, input string, options *RunJSONOptions[T]) (RunStreamedJSONResult[T], error) {
	config, err := prepareRunJSONOptions[T](options)
	if err != nil {
		return RunStreamedJSONResult[T]{}, err
	}

	if thread == nil {
		return RunStreamedJSONResult[T]{}, errors.New("RunStreamedJSON requires a non-nil thread")
	}

	raw, err := thread.runStreamed(ctx, input, nil, &config.turnOptions)
	if err != nil {
		return RunStreamedJSONResult[T]{}, err
	}

	events := make(chan ThreadEvent, runStreamedJSONEventBuffer)
	updates := make(chan RunStreamedJSONUpdate[T], runStreamedJSONEventBuffer)
	shErr := &sharedError{}

	result := RunStreamedJSONResult[T]{
		stream:  raw.stream,
		events:  events,
		updates: updates,
		err:     shErr,
	}

	go func() {
		defer close(events)
		defer close(updates)

		var deliveredFinal bool
		var turnCompleted bool

		for event := range raw.Events() {
			switch e := event.(type) {
			case ItemUpdatedEvent:
				if msg, ok := e.Item.(AgentMessageItem); ok {
					if update, decodeErr := decodeStructuredMessage[T](msg, false); decodeErr == nil {
						select {
						case updates <- update:
						case <-raw.stream.done:
							return
						default:
							// Drop intermediate snapshot when the consumer ignores updates.
						}
					}
				}
			case ItemCompletedEvent:
				if msg, ok := e.Item.(AgentMessageItem); ok {
					update, decodeErr := decodeStructuredMessage[T](msg, true)
					if decodeErr != nil {
						shErr.set(decodeErr)
					} else {
						deliveredFinal = true
						select {
						case updates <- update:
						case <-raw.stream.done:
							return
						default:
							// Drop final snapshot when the consumer ignores updates.
						}
					}
				}
			case TurnCompletedEvent:
				turnCompleted = true
			case TurnFailedEvent:
				rawErr := errors.New(e.Error.Message)
				if schemaErr, ok := classifyStructuredOutputError(rawErr, config.expectSchemaError); ok {
					shErr.set(schemaErr)
				} else {
					shErr.set(rawErr)
				}
			}

			select {
			case events <- event:
			case <-raw.stream.done:
				return
			default:
				// Drop events when no consumer is attached to avoid blocking snapshot updates.
			}
		}

		if turnCompleted && !deliveredFinal {
			shErr.set(ErrNoStructuredOutput)
		}
	}()

	return result, nil
}

type runJSONConfig struct {
	turnOptions       TurnOptions
	expectSchemaError bool
}

func prepareRunJSONOptions[T any](options *RunJSONOptions[T]) (runJSONConfig, error) {
	var config runJSONConfig

	if options != nil && options.TurnOptions != nil {
		config.turnOptions = *options.TurnOptions
	}

	var schema any
	if options != nil && options.Schema != nil {
		schema = options.Schema
	} else if config.turnOptions.OutputSchema != nil {
		schema = config.turnOptions.OutputSchema
	} else if options == nil || !options.DisableSchemaInference {
		inferred, err := inferSchemaForType[T]()
		if err != nil {
			return config, err
		}
		schema = inferred
		config.expectSchemaError = true
	} else {
		return config, errors.New("RunJSON requires a schema; provide RunJSONOptions.Schema or TurnOptions.OutputSchema")
	}

	if schema == nil {
		return config, errors.New("RunJSON resolved nil schema")
	}

	config.turnOptions.OutputSchema = schema
	if !config.expectSchemaError && schema != nil {
		config.expectSchemaError = true
	}

	return config, nil
}

func classifyStructuredOutputError(err error, expectSchema bool) (error, bool) {
	if err == nil || !expectSchema {
		return nil, false
	}
	var streamErr *ThreadStreamError
	if errors.As(err, &streamErr) {
		return nil, false
	}

	message := err.Error()
	if message == "" {
		return &SchemaViolationError{}, true
	}

	lower := strings.ToLower(message)
	if strings.Contains(lower, "schema") || strings.Contains(lower, "structured output") || strings.Contains(lower, "validation") {
		return &SchemaViolationError{Message: message}, true
	}
	return nil, false
}

func decodeStructuredMessage[T any](msg AgentMessageItem, final bool) (RunStreamedJSONUpdate[T], error) {
	var value T
	if err := json.Unmarshal([]byte(msg.Text), &value); err != nil {
		if final {
			return RunStreamedJSONUpdate[T]{}, fmt.Errorf("decode structured output: %w", err)
		}
		return RunStreamedJSONUpdate[T]{}, err
	}
	return RunStreamedJSONUpdate[T]{
		Value: value,
		Raw:   msg.Text,
		Final: final,
	}, nil
}

func inferSchemaForType[T any]() (*jsonschema.Schema, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t == nil {
		return nil, errors.New("cannot infer schema for nil type")
	}
	ref := &jsonschema.Reflector{}
	return ref.ReflectFromType(t), nil
}

type sharedError struct {
	mu  sync.Mutex
	err error
}

func (s *sharedError) set(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
}

func (s *sharedError) get() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
