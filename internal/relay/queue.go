package relay

import "sync"

// SubmissionItem is a single anonymous form response received from a respondent.
// All fields are opaque bytes — the relay never inspects the content.
type SubmissionItem struct {
	FormID             string
	EncryptedData      []byte
	EphemeralPublicKey []byte
	SchemaVersion      int32
}

// Queue is a thread-safe in-memory queue for relay submissions.
// Items are accumulated here until the flusher drains them to the database.
type Queue struct {
	mu    sync.Mutex
	items []SubmissionItem
}

// Enqueue adds an item to the queue.
func (q *Queue) Enqueue(item SubmissionItem) {
	q.mu.Lock()
	q.items = append(q.items, item)
	q.mu.Unlock()
}

// Drain atomically swaps out all queued items and returns them.
// The queue is empty after this call. Returns nil if nothing was queued.
func (q *Queue) Drain() []SubmissionItem {
	q.mu.Lock()
	items := q.items
	q.items = nil
	q.mu.Unlock()
	return items
}

// Len returns the current queue depth. Informational only.
func (q *Queue) Len() int {
	q.mu.Lock()
	n := len(q.items)
	q.mu.Unlock()
	return n
}
