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

	"github.com/observiq/blitz/generator/count"
	gennop "github.com/observiq/blitz/generator/nop"
	tracesgen "github.com/observiq/blitz/generator/traces"
	"github.com/observiq/blitz/generator/winevt"
	"github.com/observiq/blitz/internal/build"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/dispatch"
	"github.com/observiq/blitz/internal/logging"
	"github.com/observiq/blitz/internal/service"
	"github.com/observiq/blitz/internal/telemetry/metrics"
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
		Short: "A load generation tool for Bindplane managed collectors",
		Long:  "Blitz is a load generation tool for Bindplane managed collectors.",
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
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

	logger.Info("blitz started")

	// Emit Warn-level banners for any deprecated generator types
	// configured by the user. Fires once per startup, not per record.
	config.LogGeneratorDeprecations(logger, cfg)

	// Create signal context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Configure output first
	var outputInstance output.Output
	switch cfg.Output.Type {
	case config.OutputTypeNop:
		outputInstance, err = nop.New(logger)
		if err != nil {
			logger.Error("Failed to create NOP output", zap.Error(err))
			return err
		}
	case config.OutputTypeStdout:
		outputInstance, err = stdoutout.New(logger,
			stdoutout.WithFlushInterval(cfg.Output.Stdout.FlushInterval),
		)
		if err != nil {
			logger.Error("Failed to create stdout output", zap.Error(err))
			return err
		}
	case config.OutputTypeTCP:
		var tlsConfig *tls.Config
		if cfg.Output.TCP.EnableTLS {
			var tlsErr error
			tlsConfig, tlsErr = cfg.Output.TCP.TLS.Convert()
			if tlsErr != nil {
				logger.Error("Failed to convert TLS config for TCP output", zap.Error(tlsErr))
				return tlsErr
			}
		}
		outputInstance, err = tcp.New(
			logger,
			cfg.Output.TCP.Host,
			strconv.Itoa(cfg.Output.TCP.Port),
			cfg.Output.TCP.Workers,
			tlsConfig,
		)
		if err != nil {
			logger.Error("Failed to create TCP output", zap.Error(err))
			return err
		}
	case config.OutputTypeUDP:
		outputInstance, err = udp.New(
			logger,
			cfg.Output.UDP.Host,
			strconv.Itoa(cfg.Output.UDP.Port),
			cfg.Output.UDP.Workers,
		)
		if err != nil {
			logger.Error("Failed to create UDP output", zap.Error(err))
			return err
		}
	case config.OutputTypeSyslog:
		var tlsConfig *tls.Config
		if strings.ToLower(string(cfg.Output.Syslog.Transport)) == string(config.SyslogTransportTCP) && cfg.Output.Syslog.EnableTLS {
			var tlsErr error
			tlsConfig, tlsErr = cfg.Output.Syslog.TLS.Convert()
			if tlsErr != nil {
				logger.Error("Failed to convert TLS config for Syslog output", zap.Error(tlsErr))
				return tlsErr
			}
		}
		sysCfg := syslogout.Config{
			Host:             cfg.Output.Syslog.Host,
			Port:             cfg.Output.Syslog.Port,
			Transport:        syslogout.Transport(strings.ToLower(string(cfg.Output.Syslog.Transport))),
			RFC:              syslogout.RFCMode(cfg.Output.Syslog.RFC),
			Workers:          cfg.Output.Syslog.Workers,
			Facility:         cfg.Output.Syslog.Facility,
			AppName:          cfg.Output.Syslog.AppName,
			Hostname:         cfg.Output.Syslog.Hostname,
			ProcID:           cfg.Output.Syslog.ProcID,
			MsgID:            cfg.Output.Syslog.MsgID,
			MaxDatagramBytes: cfg.Output.Syslog.MaxDatagramBytes,
			TLSConfig:        tlsConfig,
		}
		outputInstance, err = syslogout.New(logger, sysCfg)
		if err != nil {
			logger.Error("Failed to create Syslog output", zap.Error(err))
			return err
		}
	case config.OutputTypeOTLPGrpc:
		opts := []otlpgrpc.OTLPGrpcOption{
			otlpgrpc.WithHost(cfg.Output.OTLPGrpc.Host),
			otlpgrpc.WithPort(strconv.Itoa(cfg.Output.OTLPGrpc.Port)),
			otlpgrpc.WithWorkers(cfg.Output.OTLPGrpc.Workers),
		}
		if cfg.Output.OTLPGrpc.BatchTimeout > 0 {
			opts = append(opts, otlpgrpc.WithBatchTimeout(cfg.Output.OTLPGrpc.BatchTimeout))
		}
		if cfg.Output.OTLPGrpc.RequestTimeout > 0 {
			opts = append(opts, otlpgrpc.WithRequestTimeout(cfg.Output.OTLPGrpc.RequestTimeout))
		}
		if cfg.Output.OTLPGrpc.MaxQueueSize > 0 {
			opts = append(opts, otlpgrpc.WithMaxQueueSize(cfg.Output.OTLPGrpc.MaxQueueSize))
		}
		if cfg.Output.OTLPGrpc.MaxExportBatchSize > 0 {
			opts = append(opts, otlpgrpc.WithMaxExportBatchSize(cfg.Output.OTLPGrpc.MaxExportBatchSize))
		}
		// Set insecure flag
		opts = append(opts, otlpgrpc.WithInsecure(cfg.Output.OTLPGrpc.Insecure))
		// If TLS is enabled and not insecure, set up TLS
		if cfg.Output.OTLPGrpc.EnableTLS && !cfg.Output.OTLPGrpc.Insecure {
			var tlsConfig *tls.Config
			tlsConfig, err = cfg.Output.OTLPGrpc.TLS.Convert()
			if err != nil {
				logger.Error("Failed to convert TLS config for OTLP gRPC output", zap.Error(err))
				return err
			}
			opts = append(opts, otlpgrpc.WithTLSConfig(tlsConfig))
		}
		outputInstance, err = otlpgrpc.New(logger, opts...)
		if err != nil {
			logger.Error("Failed to create OTLP gRPC output", zap.Error(err))
			return err
		}
	case config.OutputTypeFile:
		rot := fileout.RotationOptions{
			MaxSizeMB:  cfg.Output.File.Rotation.MaxSizeMB,
			MaxBackups: cfg.Output.File.Rotation.MaxBackups,
			MaxAgeDays: cfg.Output.File.Rotation.MaxAgeDays,
			Compress:   cfg.Output.File.Rotation.Compress,
			LocalTime:  cfg.Output.File.Rotation.LocalTime,
		}
		outputInstance, err = fileout.New(
			logger,
			cfg.Output.File.Path,
			cfg.Output.File.Workers,
			rot,
		)
		if err != nil {
			logger.Error("Failed to create File output", zap.Error(err))
			return err
		}
	case config.OutputTypeHEC:
		hecOpts := []hecout.Option{
			hecout.WithHost(cfg.Output.HEC.Host),
			hecout.WithPort(strconv.Itoa(cfg.Output.HEC.Port)),
			hecout.WithToken(cfg.Output.HEC.Token),
			hecout.WithWorkers(cfg.Output.HEC.Workers),
			hecout.WithBatchSize(cfg.Output.HEC.BatchSize),
			hecout.WithBatchTimeout(cfg.Output.HEC.BatchTimeout),
			hecout.WithEventFormat(cfg.Output.HEC.EventFormat),
			hecout.WithEnableACK(cfg.Output.HEC.EnableACK),
			hecout.WithACKPollInterval(cfg.Output.HEC.ACKPollInterval),
			hecout.WithACKTimeout(cfg.Output.HEC.ACKTimeout),
			hecout.WithMaxRetries(cfg.Output.HEC.MaxRetries),
			hecout.WithSource(cfg.Output.HEC.Source),
			hecout.WithSourceType(cfg.Output.HEC.SourceType),
			hecout.WithIndex(cfg.Output.HEC.Index),
			hecout.WithEnableTLS(cfg.Output.HEC.EnableTLS),
		}
		if cfg.Output.HEC.EnableTLS {
			var tlsConfig *tls.Config
			tlsConfig, err = cfg.Output.HEC.TLS.Convert()
			if err != nil {
				logger.Error("Failed to convert TLS config for HEC output", zap.Error(err))
				return err
			}
			hecOpts = append(hecOpts, hecout.WithTLSConfig(tlsConfig))
		}
		outputInstance, err = hecout.New(logger, hecOpts...)
		if err != nil {
			logger.Error("Failed to create HEC output", zap.Error(err))
			return err
		}
	default:
		logger.Error("Invalid output type", zap.String("type", string(cfg.Output.Type)))
		return fmt.Errorf("invalid output type: %s", cfg.Output.Type)
	}

	// Configure generators
	effectiveGens := cfg.EffectiveGenerators()
	var generators []any
	var tracker *count.Tracker

	for _, genCfg := range effectiveGens {
		gen, genErr := createGenerator(logger, genCfg, outputInstance)
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

	svc, err := service.New(logger, generators, outputInstance)
	if err != nil {
		logger.Error("Failed to create service", zap.Error(err))
		return err
	}

	if err := svc.Start(); err != nil {
		logger.Error("Failed to start service", zap.Error(err))
		return err
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

func createGenerator(logger *zap.Logger, genCfg config.Generator, out output.Output) (any, error) {
	// Standalone-CLI-only generator types that dispatch.ForEmbed does not
	// construct (winevt is deprecated for embed; nop yields no records;
	// traces is not yet migrated — lands in PIPE-1024). All other
	// generators delegate to dispatch.ForEmbed so the construction logic
	// lives in exactly one place.
	switch genCfg.Type {
	case config.GeneratorTypeNop:
		return gennop.New(logger)
	case config.GeneratorTypeWinevt:
		return winevt.New(logger, genCfg.Winevt.Workers, genCfg.Winevt.Rate)
	case config.GeneratorTypeTraces:
		tw, ok := out.(output.TraceWriter)
		if !ok {
			return nil, fmt.Errorf("traces requires an output that supports TraceWriter; configured output does not")
		}
		return tracesgen.New(tracesgen.Config{
			Logger:   logger,
			Workers:  genCfg.Traces.Workers,
			Rate:     genCfg.Traces.Rate,
			Hostname: genCfg.Traces.Hostname,
			Consumer: output.WriterAsTraceConsumer(tw),
			Seed:     genCfg.Traces.Seed,
		})
	}

	// All remaining (embed-eligible) types go through the canonical
	// dispatch.ForEmbed path. Outputs that implement MetricWriter get
	// wrapped as a MetricConsumer so metric-yielding generators
	// (hostmetrics) work standalone; ForEmbed rejects with a clear
	// message when an output doesn't support a signal the configured
	// generator needs.
	consumers := dispatch.EmbedConsumers{
		LogConsumer: output.WriterAsLogConsumer(out),
	}
	if mw, ok := out.(output.MetricWriter); ok {
		consumers.MetricConsumer = output.WriterAsMetricConsumer(mw)
	}
	mod, err := dispatch.ForEmbed(logger, genCfg, consumers, nil)
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
