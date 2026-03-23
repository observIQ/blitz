package hec

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

const (
	// ackEndpoint is the HEC ACK polling endpoint
	ackEndpoint = "/services/collector/ack"
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

// ackRequest is the request body for the ACK endpoint
type ackRequest struct {
	Acks []int64 `json:"acks"`
}

// ackResponse is the response from the ACK endpoint
type ackResponse struct {
	Acks map[string]bool `json:"acks"`
}

// resendItem carries a payload and its accumulated retry count back to the worker for resend.
type resendItem struct {
	payload    []byte
	retryCount int
}

// ackPoller polls the HEC ACK endpoint for a single worker/channel.
type ackPoller struct {
	logger     *zap.Logger
	tracker    *ackTracker
	httpClient *http.Client
	baseURL    string
	token      string
	channelID  string
	interval   time.Duration
	ackTimeout time.Duration
	maxRetries int
	resendCh   chan resendItem // channel to send payloads back for resend
	metrics    *hecMetrics
	done       chan struct{}
}

func newACKPoller(
	logger *zap.Logger,
	tracker *ackTracker,
	httpClient *http.Client,
	baseURL string,
	token string,
	channelID string,
	interval time.Duration,
	ackTimeout time.Duration,
	maxRetries int,
	resendCh chan resendItem,
	metrics *hecMetrics,
) *ackPoller {
	return &ackPoller{
		logger:     logger,
		tracker:    tracker,
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
		channelID:  channelID,
		interval:   interval,
		ackTimeout: ackTimeout,
		maxRetries: maxRetries,
		resendCh:   resendCh,
		metrics:    metrics,
		done:       make(chan struct{}),
	}
}

// run starts the ACK polling loop. It stops when stopCh is closed.
func (p *ackPoller) run(stopCh <-chan struct{}) {
	defer close(p.done)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.poll()
		case <-stopCh:
			// Final poll to confirm any remaining ackIds
			p.poll()
			return
		}
	}
}

func (p *ackPoller) poll() {
	ctx := context.Background()
	ids := p.tracker.pendingIDs()
	if len(ids) == 0 {
		return
	}

	p.metrics.recordACKPending(ctx, int64(len(ids)))

	// Query ACK status
	startTime := time.Now()
	confirmed, err := p.queryACK(ids)
	p.metrics.recordACKPollLatency(ctx, time.Since(startTime).Seconds())

	if err != nil {
		p.logger.Error("Failed to query ACK status", zap.Error(err))
		// Don't remove anything on query failure — will retry next cycle
		return
	}

	// Remove confirmed ackIds
	if len(confirmed) > 0 {
		count := p.tracker.confirm(confirmed)
		p.logger.Debug("ACKs confirmed", zap.Int("count", count))
		p.metrics.recordACKConfirmed(ctx, int64(count))
	}

	// Check for expired batches and resend
	expired := p.tracker.expired(p.ackTimeout)
	if len(expired) > 0 {
		p.metrics.recordACKExpired(ctx, int64(len(expired)))
	}

	for _, batch := range expired {
		if batch.retryCount >= p.maxRetries {
			p.logger.Warn("Dropping batch after max retries exceeded",
				zap.Int("retry_count", batch.retryCount),
				zap.Int("max_retries", p.maxRetries),
			)
			p.metrics.recordACKDropped(ctx, 1)
			continue
		}

		newRetryCount := batch.retryCount + 1
		p.logger.Warn("ACK timeout — resending batch",
			zap.Int("retry_count", newRetryCount),
			zap.Int("max_retries", p.maxRetries),
		)

		p.metrics.recordACKRetried(ctx, 1)

		// Resend the payload with the accumulated retry count
		select {
		case p.resendCh <- resendItem{payload: batch.payload, retryCount: newRetryCount}:
		default:
			p.logger.Warn("Resend channel full, dropping batch")
		}
	}

	p.metrics.recordACKPending(ctx, int64(p.tracker.size()))
}

func (p *ackPoller) queryACK(ids []int64) ([]int64, error) {
	url := p.baseURL + ackEndpoint

	reqBody := ackRequest{Acks: ids}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal ack request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ack request: %w", err)
	}

	req.Header.Set("Authorization", "Splunk "+p.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Splunk-Request-Channel", p.channelID)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send ack request: %w", err)
	}
	defer resp.Body.Close()

	var ackResp ackResponse
	if err := json.NewDecoder(resp.Body).Decode(&ackResp); err != nil {
		return nil, fmt.Errorf("decode ack response: %w", err)
	}

	var confirmed []int64
	for idStr, status := range ackResp.Acks {
		if status {
			var id int64
			if _, err := fmt.Sscanf(idStr, "%d", &id); err == nil {
				confirmed = append(confirmed, id)
			}
		}
	}

	return confirmed, nil
}
