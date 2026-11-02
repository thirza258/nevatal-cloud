package orchestrator

import (
	"sync"

	"github.com/google/uuid"
)

// Notifier wakes the SSE streams of this replica when an execution gets new
// events. Streams also poll, so events written by other replicas arrive too.
type Notifier struct {
	mu      sync.Mutex
	waiters map[uuid.UUID]chan struct{}
}

// NewNotifier returns an empty notifier.
func NewNotifier() *Notifier {
	return &Notifier{waiters: map[uuid.UUID]chan struct{}{}}
}

// Wait returns a channel that is closed at the next Notify for executionID.
// Call it before reading events so a notification between the read and the
// wait is not lost.
func (n *Notifier) Wait(executionID uuid.UUID) <-chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	ch, ok := n.waiters[executionID]
	if !ok {
		ch = make(chan struct{})
		n.waiters[executionID] = ch
	}
	return ch
}

// Notify wakes everything waiting on executionID.
func (n *Notifier) Notify(executionID uuid.UUID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ch, ok := n.waiters[executionID]; ok {
		close(ch)
		delete(n.waiters, executionID)
	}
}
