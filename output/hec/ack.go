package hec

import (
	"sync"
	"time"
)

// pendingBatch represents a batch awaiting ACK confirmation from Splunk.
type pendingBatch struct {
	payload    []byte
	sendTime   time.Time
	retryCount int
}

// ackTracker tracks pending ackIds and their associated batch payloads for a single channel.
type ackTracker struct {
	mu      sync.Mutex
	pending map[int64]*pendingBatch
}

func newACKTracker() *ackTracker {
	return &ackTracker{
		pending: make(map[int64]*pendingBatch),
	}
}

// track registers a new ackId with its batch payload.
func (a *ackTracker) track(ackID int64, payload []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending[ackID] = &pendingBatch{
		payload:  payload,
		sendTime: time.Now(),
	}
}

// trackWithRetries registers a new ackId preserving the accumulated retry count.
func (a *ackTracker) trackWithRetries(ackID int64, payload []byte, retryCount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending[ackID] = &pendingBatch{
		payload:    payload,
		sendTime:   time.Now(),
		retryCount: retryCount,
	}
}

// pendingIDs returns a copy of all pending ackIds.
func (a *ackTracker) pendingIDs() []int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]int64, 0, len(a.pending))
	for id := range a.pending {
		ids = append(ids, id)
	}
	return ids
}

// confirm removes confirmed ackIds from the tracker. Returns the count of confirmed.
func (a *ackTracker) confirm(ids []int64) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for _, id := range ids {
		if _, ok := a.pending[id]; ok {
			delete(a.pending, id)
			count++
		}
	}
	return count
}

// expired returns batches that have exceeded the ack timeout and removes them from the tracker.
func (a *ackTracker) expired(timeout time.Duration) []*pendingBatch {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	var result []*pendingBatch
	for id, batch := range a.pending {
		if now.Sub(batch.sendTime) > timeout {
			result = append(result, batch)
			delete(a.pending, id)
		}
	}
	return result
}

// size returns the number of pending ackIds.
func (a *ackTracker) size() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}
