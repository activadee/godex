package godex

import (
	"context"
	"sync"
)

// Stream is an internal helper that coordinates the lifecycle of a streaming turn.
type Stream struct {
	events <-chan ThreadEvent
	cancel context.CancelFunc

	done chan struct{}

	mu  sync.Mutex
	err error
}

func newStream(events <-chan ThreadEvent, cancel context.CancelFunc) *Stream {
	return &Stream{
		events: events,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

func (s *Stream) Events() <-chan ThreadEvent {
	return s.events
}

func (s *Stream) setErr(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	close(s.done)
}

func (s *Stream) finish() {
	// finish is called when the producer goroutine exits to close the done channel in case
	// setErr was not invoked (should not happen, but defensive).
	select {
	case <-s.done:
		return
	default:
		s.setErr(nil)
	}
}

func (s *Stream) Wait() error {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Stream) Close() error {
	s.cancel()
	return s.Wait()
}
