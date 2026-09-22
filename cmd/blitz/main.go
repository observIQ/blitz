// Package main is the main package for the Bindplane Loader.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/filegen/embeddedlibrary"
	gennop "github.com/observiq/blitz/generator/nop"
	"github.com/observiq/blitz/generator/winevt"
	"github.com/observiq/blitz/internal/build"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/internal/dispatch"
	"github.com/observiq/blitz/internal/logging"
	"github.com/observiq/blitz/internal/runtime"
	"github.com/observiq/blitz/internal/service"
	"github.com/observiq/blitz/internal/telemetry/logs"
	"github.com/observiq/blitz/internal/telemetry/metrics"
	"github.com/observiq/blitz/internal/telemetry/traces"
	"github.com/observiq/blitz/output"
	fileout "github.com/observiq/blitz/output/file"
	hecout "github.com/observiq/blitz/output/hec"
	"github.com/observiq/blitz/output/nop"
	otlpgrpc "github.com/observiq/blitz/output/otlp_grpc"
	stdoutout "github.com/observiq/blitz/output/stdout"
	syslogout "github.com/observiq/blitz/output/syslog"
	"github.com/observiq/blitz/output/tcp"
	"github.com/observiq/blitz/output/udp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "blitz",
		Short: "The last, best, most magnificent telemetry generation/simulation tool anyone will ever need",
		Long:  "Blitz is the last, best, most magnificent telemetry generation/simulation tool anyone will ever need.",
		RunE:  run,
	}
)

func init() {
	// Add config file flag
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "path to configuration file")

	// Bind all configuration overrides to flags
	for _, override := range config.DefaultOverrides() {
		if err := override.Bind(rootCmd.PersistentFlags()); err != nil {
			fmt.Printf("Failed to bind override %s: %s", override.Field, err.Error())
			os.Exit(1)
		}
	}

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			info := build.GetInfo()
			jsonData, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				fmt.Printf("Failed to marshal version info: %s\n", err.Error())
				os.Exit(1)
			}
			fmt.Println(string(jsonData))
		},
	})

	// Add completion command
	rootCmd.AddCommand(newCompletionCommand())

	// Add library command group
	rootCmd.AddCommand(newLibraryCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Mark process start so the blitz.startup.duration metric can measure the
	// full standalone startup: config load, provider construction, output
	// wiring, and bringing every generator up.
	startTime := time.Now()

	// Configure Viper to handle env overrides
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read configuration file if provided
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file %s: %w", configFile, err)
		}
	}

	config.MigrateDeprecatedKeys(viper.GetViper())

	if err := config.ValidateExclusiveOutputConfig(viper.GetViper()); err != nil {
		return err
	}

	cfg := config.NewConfig()
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}

	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	// Create signal context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build blitz's self-telemetry bundle. The log provider is constructed and
	// the logger bridged FIRST, before any startup logging, so every line from
	// here on is exported when log export is enabled. blitz bridges the logger
	// once, here at the process entry point, and shares the bridged logger with
	// every component; components do not re-bridge, since a bridged zap logger
	// propagates to the child loggers they derive. Metrics leave the provider
	// nil so they fall back to the process-global Prometheus provider that
	// setupMetrics installs.
	tel := embed.TelemetrySettings{PerBatchSpans: cfg.Telemetry.Traces.PerBatchSpans}
	logExportEnabled := cfg.Telemetry.Logs.OTLPEndpoint != ""
	if logExportEnabled {
		otlpLogs, lerr := logs.NewOTLP(ctx, cfg.Telemetry.Logs.OTLPEndpoint, cfg.Telemetry.Logs.Insecure)
		if lerr != nil {
			logger.Error("Failed to enable self-telemetry log export", zap.Error(lerr))
			return lerr
		}
		defer func() { _ = otlpLogs.Shutdown(context.Background()) }()
		tel.LoggerProvider = otlpLogs.Provider()
	}
	logger = tel.BridgedLogger(logger)
	tel.Logger = logger

	logger.Info("blitz started")
	if logExportEnabled {
		logger.Info("self-telemetry log export enabled",
			zap.String("endpoint", cfg.Telemetry.Logs.OTLPEndpoint))
	}

	// Emit Warn-level banners for any deprecated generator types
	// configured by the user. Fires once per startup, not per record.
	config.LogGeneratorDeprecations(logger, cfg)

	if err := setupMetrics(ctx, cfg, logger); err != nil {
		logger.Error("Failed to setup metrics", zap.Error(err))
		return err
	}

	// Listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received signal, initiating graceful shutdown", zap.String("signal", sig.String()))
		cancel()
	}()

	// Trace export is opt-in via the telemetry.traces config: when an OTLP
	// endpoint is set, spans export there; otherwise the nil TracerProvider
	// means spans are created but dropped by the global no-op provider.
	if cfg.Telemetry.Traces.OTLPEndpoint != "" {
		otlpTraces, terr := traces.NewOTLP(ctx, cfg.Telemetry.Traces.OTLPEndpoint, cfg.Telemetry.Traces.Insecure)
		if terr != nil {
			logger.Error("Failed to enable self-telemetry trace export", zap.Error(terr))
			return terr
		}
		defer func() { _ = otlpTraces.Shutdown(context.Background()) }()
		tel.TracerProvider = otlpTraces.Provider()
		logger.Info("self-telemetry trace export enabled",
			zap.String("endpoint", cfg.Telemetry.Traces.OTLPEndpoint))
	}

	// Build outputs; fan out to all when more than one is configured.
	outputInstance, outSignals, err := buildOutputs(logger, cfg.EffectiveOutputs(), tel)
	if err != nil {
		return err
	}

	// Build the simulated identity environment once, up front, so every
	// generator resolves its host identity from the same fleet (PIPE-1036). A
	// live-reconfigure path (deferred) would rebuild this and swap it in.
	env, err := cfg.Environment.Build(logger)
	if err != nil {
		logger.Error("Failed to build simulated environment", zap.Error(err))
		return err
	}

	// Configure generators
	effectiveGens := cfg.EffectiveGenerators()

	// Fail fast on a signal mismatch, reporting every orphan at once: a
	// generated signal with no accepting output, or an output that accepts no
	// generated signal.
	genTypes := make([]config.GeneratorType, 0, len(effectiveGens))
	for _, g := range effectiveGens {
		genTypes = append(genTypes, g.Type)
	}
	if err := dispatch.ValidateSignalCompat(genTypes, outSignals); err != nil {
		logger.Error("output/generator signal mismatch", zap.Error(err))
		return fmt.Errorf("signal compatibility: %w", err)
	}

	var generators []any
	var tracker *count.Tracker

	for _, genCfg := range effectiveGens {
		gen, genErr := createGenerator(logger, genCfg, outputInstance, env, tel)
		if genErr != nil {
			logger.Error("Failed to create generator",
				zap.String("type", string(genCfg.Type)),
				zap.Error(genErr))
			return genErr
		}

		// Set up finite generation count tracker
		if genCfg.Count > 0 {
			if tracker == nil {
				tracker = count.NewTracker(int64(genCfg.Count))
				logger.Info("Finite generation enabled",
					zap.Int("count", genCfg.Count),
					zap.String("onFinish", cfg.OnFinish))
			}
			if s, ok := gen.(interface{ SetCountTracker(*count.Tracker) }); ok {
				s.SetCountTracker(tracker)
			}
		}

		generators = append(generators, gen)
	}

	// Set up SIGUSR1 restart signal handler
	setupRestartSignal(ctx, logger, tracker)

	svc, err := service.New(logger, generators, outputInstance, tel)
	if err != nil {
		logger.Error("Failed to create service", zap.Error(err))
		return err
	}

	if err := svc.Start(); err != nil {
		logger.Error("Failed to start service", zap.Error(err))
		return err
	}

	// Record process-level startup latency (best effort; a metric-build failure
	// must not fail startup). Per-module and session startup are recorded by the
	// runtime.
	if lifecycleMetrics, merr := runtime.NewMetrics(tel.MeterProvider); merr == nil {
		lifecycleMetrics.BlitzStartupDurationHistogram.Record(ctx, runtime.DurationMillis(time.Since(startTime)))
	}

	if tracker == nil {
		<-ctx.Done()
	} else if cfg.OnFinish == "idle" {
		for {
			select {
			case <-ctx.Done():
				goto shutdown
			case <-tracker.Done():
				logger.Info("Generation count reached, idling (SIGUSR1 to restart)")
			}
			select {
			case <-ctx.Done():
				goto shutdown
			case <-tracker.ResumeC():
				logger.Info("Generation restarted")
			}
		}
	} else {
		select {
		case <-ctx.Done():
		case <-tracker.Done():
			logger.Info("Generation count reached, exiting")
		}
	}
shutdown:

	if err := svc.Stop(); err != nil {
		logger.Error("Failed to stop service", zap.Error(err))
		return err
	}

	if tracker != nil {
		logger.Info("Telemetry generation completed",
			zap.String("generator", string(cfg.Generator.Type)),
			zap.Int64("records_emitted", tracker.Emitted()))
	}

	logger.Info("blitz shutdown complete")
	return nil
}

func createGenerator(logger *zap.Logger, genCfg config.Generator, out output.Output, env *datagen.Environment, tel embed.TelemetrySettings) (any, error) {
	// Standalone-CLI-only generator types that dispatch.ForEmbed does not
	// construct (winevt is deprecated for embed; nop yields no records).
	// All other generators delegate to dispatch.ForEmbed so the
	// construction logic lives in exactly one place.
	switch genCfg.Type {
	case config.GeneratorTypeNop:
		return gennop.New(logger)
	case config.GeneratorTypeWinevt:
		return winevt.New(logger, genCfg.Winevt.Workers, genCfg.Winevt.Rate, tel)
	}

	// All remaining (embed-eligible) types go through the canonical
	// dispatch.ForEmbed path. Outputs that implement MetricWriter or
	// TraceWriter get wrapped as the corresponding consumer so
	// metric-yielding (hostmetrics) and trace-yielding (traces)
	// generators work standalone; ForEmbed rejects with a clear message
	// when an output doesn't support a signal the configured generator
	// needs.
	consumers := dispatch.EmbedConsumers{}
	if lw, ok := out.(output.LogWriter); ok {
		consumers.LogConsumer = output.WriterAsLogConsumer(lw, tel)
	}
	if mw, ok := out.(output.MetricWriter); ok {
		consumers.MetricConsumer = output.WriterAsMetricConsumer(mw, tel)
	}
	if tw, ok := out.(output.TraceWriter); ok {
		consumers.TraceConsumer = output.WriterAsTraceConsumer(tw, tel)
	}
	// Pass the embedded library so an embed_library build resolves package
	// sources; without the tag FS() is empty and resolution uses disk (PIPE-1445).
	mod, err := dispatch.ForEmbed(logger, genCfg, consumers, embeddedlibrary.FS(), env, tel)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

func setupMetrics(ctx context.Context, cfg *config.Config, logger *zap.Logger) error {
	logger.Info("starting metrics server")

	prometheus, err := metrics.NewPrometheus()
	if err != nil {
		return fmt.Errorf("new prometheus: %w", err)
	}

	if err := prometheus.Start(ctx); err != nil {
		return fmt.Errorf("start prometheus exporter: %w", err)
	}

	go func() {
		err := httpServer(cfg.Metrics.Port, logger)
		if err != nil {
			logger.Error("http server", zap.Error(err))
		}
	}()

	logger.Info("metrics server started")

	return nil
}

func httpServer(port int, logger *zap.Logger) error {
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))

	s := &http.Server{
		Addr:              addr,
		IdleTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	s.Handler = promhttp.Handler()

	logger.Info("starting metrics HTTP server", zap.String("addr", addr))
	return s.ListenAndServe()
}

// buildOutput constructs a single output from its config.
func buildOutput(logger *zap.Logger, out config.Output, tel embed.TelemetrySettings) (output.Output, error) {
	var inst output.Output
	var err error
	switch out.Type {
	case config.OutputTypeNop:
		inst, err = nop.New(logger, tel)
		if err != nil {
			logger.Error("Failed to create NOP output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeStdout:
		inst, err = stdoutout.New(logger,
			stdoutout.WithFlushInterval(out.Stdout.FlushInterval),
			stdoutout.WithTelemetry(tel),
		)
		if err != nil {
			logger.Error("Failed to create stdout output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeTCP:
		var tlsConfig *tls.Config
		if out.TCP.EnableTLS {
			var tlsErr error
			tlsConfig, tlsErr = out.TCP.TLS.Convert()
			if tlsErr != nil {
				logger.Error("Failed to convert TLS config for TCP output", zap.Error(tlsErr))
				return nil, tlsErr
			}
		}
		inst, err = tcp.New(
			logger,
			out.TCP.Host,
			strconv.Itoa(out.TCP.Port),
			out.TCP.Workers,
			tlsConfig,
			tel,
		)
		if err != nil {
			logger.Error("Failed to create TCP output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeUDP:
		inst, err = udp.New(
			logger,
			out.UDP.Host,
			strconv.Itoa(out.UDP.Port),
			out.UDP.Workers,
			tel,
		)
		if err != nil {
			logger.Error("Failed to create UDP output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeSyslog:
		var tlsConfig *tls.Config
		if strings.ToLower(string(out.Syslog.Transport)) == string(config.SyslogTransportTCP) && out.Syslog.EnableTLS {
			var tlsErr error
			tlsConfig, tlsErr = out.Syslog.TLS.Convert()
			if tlsErr != nil {
				logger.Error("Failed to convert TLS config for Syslog output", zap.Error(tlsErr))
				return nil, tlsErr
			}
		}
		sysCfg := syslogout.Config{
			Host:             out.Syslog.Host,
			Port:             out.Syslog.Port,
			Transport:        syslogout.Transport(strings.ToLower(string(out.Syslog.Transport))),
			RFC:              syslogout.RFCMode(out.Syslog.RFC),
			Workers:          out.Syslog.Workers,
			Facility:         out.Syslog.Facility,
			AppName:          out.Syslog.AppName,
			Hostname:         out.Syslog.Hostname,
			ProcID:           out.Syslog.ProcID,
			MsgID:            out.Syslog.MsgID,
			MaxDatagramBytes: out.Syslog.MaxDatagramBytes,
			TLSConfig:        tlsConfig,
			Telemetry:        tel,
		}
		inst, err = syslogout.New(logger, sysCfg)
		if err != nil {
			logger.Error("Failed to create Syslog output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeOTLPGrpc:
		opts := []otlpgrpc.OTLPGrpcOption{
			otlpgrpc.WithHost(out.OTLPGrpc.Host),
			otlpgrpc.WithPort(strconv.Itoa(out.OTLPGrpc.Port)),
			otlpgrpc.WithWorkers(out.OTLPGrpc.Workers),
		}
		if out.OTLPGrpc.BatchTimeout > 0 {
			opts = append(opts, otlpgrpc.WithBatchTimeout(out.OTLPGrpc.BatchTimeout))
		}
		if out.OTLPGrpc.RequestTimeout > 0 {
			opts = append(opts, otlpgrpc.WithRequestTimeout(out.OTLPGrpc.RequestTimeout))
		}
		if out.OTLPGrpc.MaxQueueSize > 0 {
			opts = append(opts, otlpgrpc.WithMaxQueueSize(out.OTLPGrpc.MaxQueueSize))
		}
		if out.OTLPGrpc.MaxExportBatchSize > 0 {
			opts = append(opts, otlpgrpc.WithMaxExportBatchSize(out.OTLPGrpc.MaxExportBatchSize))
		}
		// Set insecure flag
		opts = append(opts, otlpgrpc.WithInsecure(out.OTLPGrpc.Insecure))
		opts = append(opts, otlpgrpc.WithTelemetry(tel))
		// If TLS is enabled and not insecure, set up TLS
		if out.OTLPGrpc.EnableTLS && !out.OTLPGrpc.Insecure {
			var tlsConfig *tls.Config
			tlsConfig, err = out.OTLPGrpc.TLS.Convert()
			if err != nil {
				logger.Error("Failed to convert TLS config for OTLP gRPC output", zap.Error(err))
				return nil, err
			}
			opts = append(opts, otlpgrpc.WithTLSConfig(tlsConfig))
		}
		inst, err = otlpgrpc.New(logger, opts...)
		if err != nil {
			logger.Error("Failed to create OTLP gRPC output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeFile:
		rot := fileout.RotationOptions{
			MaxSizeMB:  out.File.Rotation.MaxSizeMB,
			MaxBackups: out.File.Rotation.MaxBackups,
			MaxAgeDays: out.File.Rotation.MaxAgeDays,
			Compress:   out.File.Rotation.Compress,
			LocalTime:  out.File.Rotation.LocalTime,
		}
		inst, err = fileout.New(
			logger,
			out.File.Path,
			out.File.Workers,
			rot,
			tel,
		)
		if err != nil {
			logger.Error("Failed to create File output", zap.Error(err))
			return nil, err
		}
	case config.OutputTypeHEC:
		hecOpts := []hecout.Option{
			hecout.WithHost(out.HEC.Host),
			hecout.WithPort(strconv.Itoa(out.HEC.Port)),
			hecout.WithToken(out.HEC.Token),
			hecout.WithWorkers(out.HEC.Workers),
			hecout.WithBatchSize(out.HEC.BatchSize),
			hecout.WithBatchTimeout(out.HEC.BatchTimeout),
			hecout.WithEventFormat(out.HEC.EventFormat),
			hecout.WithEnableACK(out.HEC.EnableACK),
			hecout.WithACKPollInterval(out.HEC.ACKPollInterval),
			hecout.WithACKTimeout(out.HEC.ACKTimeout),
			hecout.WithMaxRetries(out.HEC.MaxRetries),
			hecout.WithSource(out.HEC.Source),
			hecout.WithSourceType(out.HEC.SourceType),
			hecout.WithIndex(out.HEC.Index),
			hecout.WithEnableTLS(out.HEC.EnableTLS),
			hecout.WithTelemetry(tel),
		}
		if out.HEC.EnableTLS {
			var tlsConfig *tls.Config
			tlsConfig, err = out.HEC.TLS.Convert()
			if err != nil {
				logger.Error("Failed to convert TLS config for HEC output", zap.Error(err))
				return nil, err
			}
			hecOpts = append(hecOpts, hecout.WithTLSConfig(tlsConfig))
		}
		inst, err = hecout.New(logger, hecOpts...)
		if err != nil {
			logger.Error("Failed to create HEC output", zap.Error(err))
			return nil, err
		}
	default:
		logger.Error("Invalid output type", zap.String("type", string(out.Type)))
		return nil, fmt.Errorf("invalid output type: %s", out.Type)
	}
	return inst, nil
}

// buildOutputs builds each output; wraps in MultiOutput when >1 (fan-out).
// It also returns per-output signals so validation sees each destination
// individually, catching an output that would receive nothing.
func buildOutputs(logger *zap.Logger, outs []config.Output, tel embed.TelemetrySettings) (output.Output, []dispatch.OutputSignals, error) {
	built := make([]output.Output, 0, len(outs))
	signals := make([]dispatch.OutputSignals, 0, len(outs))
	for _, oc := range outs {
		o, err := buildOutput(logger, oc, tel)
		if err != nil {
			return nil, nil, err
		}
		built = append(built, o)
		signals = append(signals, dispatch.OutputSignals{Name: string(oc.Type), Signals: o.SupportedTelemetry()})
	}
	if len(built) == 1 {
		return built[0], signals, nil
	}
	logger.Info("multi-output fan-out enabled", zap.Int("outputs", len(built)))
	return output.NewMultiOutput(built...), signals, nil
}
