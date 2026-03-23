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
	metrics  *hecMetrics
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

	// Metrics
	metrics *hecMetrics

	// ACK support
	tracker    *ackTracker
	poller     *ackPoller
	pollerStop chan struct{}
	resendCh   chan resendItem
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

	m, err := newHECMetrics()
	if err != nil {
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &HEC{
		logger:   logger.Named("output-hec"),
		cfg:      cfg,
		hostname: hostname,
		dataChan: make(chan output.LogRecord, DefaultChannelSize),
		ctx:      ctx,
		cancel:   cancel,
		metrics:  m,
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
		w, err := newWorker(i, h.logger, cfg, hostname, h.dataChan, ctx, m)
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

	m.recordActiveWorkers(context.Background(), int64(cfg.workers))

	return h, nil
}

// Write sends data to the HEC output channel for processing by workers.
func (h *HEC) Write(ctx context.Context, data output.LogRecord) error {
	select {
	case h.dataChan <- data:
		h.metrics.recordLogsReceived(ctx, 1)
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
	h.metrics.recordActiveWorkers(ctx, 0)

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

func newWorker(id int, logger *zap.Logger, cfg Config, hostname string, dataChan chan output.LogRecord, ctx context.Context, m *hecMetrics) (*worker, error) {
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
		metrics:  m,
	}

	// Set up ACK tracking if enabled
	if cfg.enableACK {
		w.tracker = newACKTracker()
		w.pollerStop = make(chan struct{})
		w.resendCh = make(chan resendItem, 100)
		w.poller = newACKPoller(
			w.logger,
			w.tracker,
			w.httpClient,
			baseURL,
			cfg.token,
			channelID,
			cfg.ackPollInterval,
			cfg.ackTimeout,
			cfg.maxRetries,
			w.resendCh,
			m,
		)
	}

	return w, nil
}

func (w *worker) run() {
	defer close(w.done)
	w.logger.Info("Starting HEC worker")

	// Start ACK poller if enabled
	if w.poller != nil {
		go w.poller.run(w.pollerStop)
		go w.processResends()
	}

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
				w.stopACKPoller()
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

// processResends handles resend payloads from the ACK poller.
// These are pre-serialized batch payloads that need to be re-POSTed to HEC.
func (w *worker) processResends() {
	for item := range w.resendCh {
		resp, err := w.postEvents(item.payload)
		if err != nil {
			w.logger.Error("Failed to resend batch", zap.Error(err))
			continue
		}
		if resp.Code != 0 {
			w.logger.Error("HEC returned error on resend",
				zap.Int("code", resp.Code),
				zap.String("text", resp.Text),
			)
			continue
		}
		// Track the new ackId from the resend, preserving the accumulated retry count
		if w.tracker != nil && resp.AckID != nil {
			w.tracker.trackWithRetries(*resp.AckID, item.payload, item.retryCount)
		}
	}
}

// stopACKPoller stops the ACK poller and waits for it to finish, then closes the resend channel.
func (w *worker) stopACKPoller() {
	if w.pollerStop == nil {
		return
	}
	close(w.pollerStop)
	<-w.poller.done
	close(w.resendCh)
}

func (w *worker) sendBatch(batch []output.LogRecord) {
	ctx := context.Background()
	batchLen := int64(len(batch))
	w.metrics.recordBatchSize(ctx, batchLen)

	payload, err := w.buildPayload(batch)
	if err != nil {
		w.logger.Error("Failed to build HEC payload", zap.Error(err))
		w.metrics.recordSendError(ctx, "encode")
		return
	}

	startTime := time.Now()
	resp, err := w.postEvents(payload)
	latency := time.Since(startTime).Seconds()

	if err != nil {
		w.logger.Error("Failed to send HEC events", zap.Error(err), zap.Int("batch_size", len(batch)))
		w.metrics.recordSendError(ctx, "transport")
		return
	}

	w.metrics.recordRequestLatency(ctx, latency)
	w.metrics.recordRequestSize(ctx, int64(len(payload)))

	if resp.Code != 0 {
		w.logger.Error("HEC returned error",
			zap.Int("code", resp.Code),
			zap.String("text", resp.Text),
			zap.Int("batch_size", len(batch)),
		)
		w.metrics.recordSendError(ctx, "hec_error")
		return
	}

	w.metrics.recordLogRate(ctx, float64(batchLen))

	// Track the ackId for ACK confirmation
	if w.tracker != nil && resp.AckID != nil {
		w.tracker.track(*resp.AckID, payload)
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Splunk "+w.cfg.token)
	req.Header.Set("Content-Type", "application/json")
	if w.cfg.enableACK {
		req.Header.Set("X-Splunk-Request-Channel", w.channelID)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var hecResp hecResponse
	if err := json.NewDecoder(resp.Body).Decode(&hecResp); err != nil {
		return nil, fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
	}

	return &hecResp, nil
}
