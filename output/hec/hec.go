package hec

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/goccy/go-json"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

const (
	// DefaultChannelSize is the default size of the data channel
	DefaultChannelSize = 100

	// DefaultHTTPTimeout is the default timeout for HTTP requests
	DefaultHTTPTimeout = 30 * time.Second

	// eventEndpoint is the HEC event submission endpoint
	eventEndpoint = "/services/collector/event"
)

// hecEvent represents a single HEC event payload
type hecEvent struct {
	Time       float64 `json:"time"`
	Host       string  `json:"host"`
	Source     string  `json:"source,omitempty"`
	SourceType string  `json:"sourcetype,omitempty"`
	Index      string  `json:"index,omitempty"`
	Event      any     `json:"event"`
}

// hecResponse represents the response from HEC event submission
type hecResponse struct {
	Text  string `json:"text"`
	Code  int    `json:"code"`
	AckID *int64 `json:"ackId,omitempty"`
}

// HEC implements the Output interface for Splunk HTTP Event Collector
type HEC struct {
	logger   *zap.Logger
	cfg      Config
	hostname string
	dataChan chan output.LogRecord
	ctx      context.Context
	cancel   context.CancelFunc
	workers  []*worker
}

// worker represents a single HEC worker with its own channel UUID and HTTP client
type worker struct {
	id         int
	logger     *zap.Logger
	cfg        Config
	hostname   string
	channelID  string
	httpClient *http.Client
	baseURL    string
	dataChan   chan output.LogRecord
	ctx        context.Context
	done       chan struct{}
}

// New creates a new HEC output instance
func New(logger *zap.Logger, opts ...Option) (*HEC, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	cfg := Config{
		workers:         config.DefaultHECWorkers,
		batchSize:       config.DefaultHECBatchSize,
		batchTimeout:    config.DefaultHECBatchTimeout,
		eventFormat:     config.DefaultHECEventFormat,
		enableACK:       config.DefaultHECEnableACK,
		ackPollInterval: config.DefaultHECACKPollInterval,
		ackTimeout:      config.DefaultHECACKTimeout,
		maxRetries:      config.DefaultHECMaxRetries,
		source:          config.DefaultHECSource,
		sourceType:      config.DefaultHECSourceType,
		enableTLS:       config.DefaultHECEnableTLS,
	}

	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	if cfg.host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if cfg.port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}
	if cfg.token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}
	if cfg.workers <= 0 {
		cfg.workers = config.DefaultHECWorkers
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &HEC{
		logger:   logger.Named("output-hec"),
		cfg:      cfg,
		hostname: hostname,
		dataChan: make(chan output.LogRecord, DefaultChannelSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	h.logger.Info("Starting HEC output",
		zap.String("host", cfg.host),
		zap.String("port", cfg.port),
		zap.Int("workers", cfg.workers),
		zap.Int("batch_size", cfg.batchSize),
		zap.Duration("batch_timeout", cfg.batchTimeout),
		zap.String("event_format", cfg.eventFormat),
		zap.Bool("enable_ack", cfg.enableACK),
		zap.Bool("enable_tls", cfg.enableTLS),
	)

	// Create workers
	for i := range cfg.workers {
		w, err := newWorker(i, h.logger, cfg, hostname, h.dataChan, ctx)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create worker %d: %w", i, err)
		}
		h.workers = append(h.workers, w)
	}

	// Start workers
	for _, w := range h.workers {
		go w.run()
	}

	return h, nil
}

// Write sends data to the HEC output channel for processing by workers.
func (h *HEC) Write(ctx context.Context, data output.LogRecord) error {
	select {
	case h.dataChan <- data:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write data: %w", ctx.Err())
	case <-h.ctx.Done():
		return fmt.Errorf("HEC output is shutting down")
	}
}

// Stop gracefully shuts down all workers
func (h *HEC) Stop(ctx context.Context) error {
	h.logger.Info("Stopping HEC output")

	// Close the channel to signal workers to drain remaining items and stop.
	// Do NOT cancel the context yet — workers need it for in-flight HTTP requests
	// during the drain phase.
	close(h.dataChan)

	// Wait for all workers to finish draining
	for _, w := range h.workers {
		select {
		case <-w.done:
		case <-ctx.Done():
			h.logger.Warn("Timed out waiting for worker to stop", zap.Int("worker_id", w.id))
		}
	}

	// Now cancel the context (cleanup)
	h.cancel()

	h.logger.Info("HEC output stopped successfully")
	return nil
}

func newWorker(id int, logger *zap.Logger, cfg Config, hostname string, dataChan chan output.LogRecord, ctx context.Context) (*worker, error) {
	scheme := "https"
	if !cfg.enableTLS {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(cfg.host, cfg.port))

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 10,
	}
	if cfg.tlsConfig != nil {
		transport.TLSClientConfig = cfg.tlsConfig
	} else if cfg.enableTLS {
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	channelID := generateChannelID()

	w := &worker{
		id:        id,
		logger:    logger.With(zap.Int("worker_id", id), zap.String("channel_id", channelID)),
		cfg:       cfg,
		hostname:  hostname,
		channelID: channelID,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   DefaultHTTPTimeout,
		},
		baseURL:  baseURL,
		dataChan: dataChan,
		ctx:      ctx,
		done:     make(chan struct{}),
	}

	return w, nil
}

func (w *worker) run() {
	defer close(w.done)
	w.logger.Info("Starting HEC worker")

	batch := make([]output.LogRecord, 0, w.cfg.batchSize)
	timer := time.NewTimer(w.cfg.batchTimeout)
	defer timer.Stop()

	for {
		select {
		case record, ok := <-w.dataChan:
			if !ok {
				// Channel closed — flush remaining batch and exit
				if len(batch) > 0 {
					w.sendBatch(batch)
				}
				w.logger.Info("HEC worker exiting - channel closed")
				return
			}

			batch = append(batch, record)
			if len(batch) >= w.cfg.batchSize {
				w.sendBatch(batch)
				batch = batch[:0]
				timer.Reset(w.cfg.batchTimeout)
			}

		case <-timer.C:
			if len(batch) > 0 {
				w.sendBatch(batch)
				batch = batch[:0]
			}
			timer.Reset(w.cfg.batchTimeout)
		}
	}
}

func (w *worker) sendBatch(batch []output.LogRecord) {
	payload, err := w.buildPayload(batch)
	if err != nil {
		w.logger.Error("Failed to build HEC payload", zap.Error(err))
		return
	}

	resp, err := w.postEvents(payload)
	if err != nil {
		w.logger.Error("Failed to send HEC events", zap.Error(err), zap.Int("batch_size", len(batch)))
		return
	}

	if resp.Code != 0 {
		w.logger.Error("HEC returned error",
			zap.Int("code", resp.Code),
			zap.String("text", resp.Text),
			zap.Int("batch_size", len(batch)),
		)
		return
	}

	w.logger.Debug("HEC batch sent successfully",
		zap.Int("batch_size", len(batch)),
		zap.Int64p("ack_id", resp.AckID),
	)
}

func (w *worker) buildPayload(batch []output.LogRecord) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	for _, record := range batch {
		event := w.formatEvent(record)
		if err := encoder.Encode(event); err != nil {
			return nil, fmt.Errorf("encode event: %w", err)
		}
	}

	return buf.Bytes(), nil
}

func (w *worker) formatEvent(record output.LogRecord) hecEvent {
	var eventData any

	if w.cfg.eventFormat == config.HECEventFormatParsed && record.ParseFunc != nil {
		parsed, err := record.ParseFunc(record.Message)
		if err == nil {
			eventData = parsed
		} else {
			// Fall back to raw on parse error
			eventData = record.Message
		}
	} else {
		eventData = record.Message
	}

	timestamp := float64(record.Metadata.Timestamp.UnixMilli()) / 1000.0
	if record.Metadata.Timestamp.IsZero() {
		timestamp = float64(time.Now().UnixMilli()) / 1000.0
	}

	return hecEvent{
		Time:       timestamp,
		Host:       w.hostname,
		Source:     w.cfg.source,
		SourceType: w.cfg.sourceType,
		Index:      w.cfg.index,
		Event:      eventData,
	}
}

func (w *worker) postEvents(payload []byte) (*hecResponse, error) {
	url := w.baseURL + eventEndpoint

	var hecResp hecResponse

	b := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(100*time.Millisecond),
		backoff.WithMaxInterval(30*time.Second),
		backoff.WithMaxElapsedTime(2*time.Minute),
		backoff.WithMultiplier(2),
		backoff.WithRandomizationFactor(0.1),
	)

	operation := func() error {
		req, err := http.NewRequestWithContext(w.ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return backoff.Permanent(fmt.Errorf("create request: %w", err))
		}

		req.Header.Set("Authorization", "Splunk "+w.cfg.token)
		req.Header.Set("Content-Type", "application/json")
		if w.cfg.enableACK {
			req.Header.Set("X-Splunk-Request-Channel", w.channelID)
		}

		resp, err := w.httpClient.Do(req)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("send request: %w", err))
		}

		err = json.NewDecoder(resp.Body).Decode(&hecResp)
		resp.Body.Close()
		if err != nil {
			return backoff.Permanent(fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err))
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			return nil
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return backoff.Permanent(fmt.Errorf("auth error (HTTP %d): %s", resp.StatusCode, hecResp.Text))
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
			w.logger.Warn("HEC server busy, retrying",
				zap.Int("status", resp.StatusCode),
				zap.String("text", hecResp.Text),
			)
			return fmt.Errorf("server busy (HTTP %d)", resp.StatusCode)
		default:
			return backoff.Permanent(fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, hecResp.Text))
		}
	}

	if err := backoff.Retry(operation, backoff.WithContext(b, w.ctx)); err != nil {
		return nil, err
	}

	return &hecResp, nil
}
