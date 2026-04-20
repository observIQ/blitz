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

	"github.com/observiq/blitz/generator"
	apachegen "github.com/observiq/blitz/generator/apache"
	apachecombinedgen "github.com/observiq/blitz/generator/apache_combined"
	apacheerrorgen "github.com/observiq/blitz/generator/apache_error"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/filegen"
	jsongen "github.com/observiq/blitz/generator/json"
	"github.com/observiq/blitz/generator/kubernetes"
	"github.com/observiq/blitz/generator/nginx"
	gennop "github.com/observiq/blitz/generator/nop"
	"github.com/observiq/blitz/generator/okta"
	"github.com/observiq/blitz/generator/paloalto"
	"github.com/observiq/blitz/generator/postgres"
	"github.com/observiq/blitz/generator/winevt"
	"github.com/observiq/blitz/internal/build"
	"github.com/observiq/blitz/internal/config"
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
		outputInstance, err = stdoutout.New(logger)
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

	// Configure generator
	var generatorInstance generator.Generator
	switch cfg.Generator.Type {
	case config.GeneratorTypeNop:
		generatorInstance, err = gennop.New(logger)
		if err != nil {
			logger.Error("Failed to create NOP generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeJSON:
		generatorInstance, err = jsongen.New(
			logger,
			cfg.Generator.JSON.Workers,
			cfg.Generator.JSON.Rate,
			cfg.Generator.JSON.Type,
		)
		if err != nil {
			logger.Error("Failed to create JSON generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeWinevt:
		generatorInstance, err = winevt.New(
			logger,
			cfg.Generator.Winevt.Workers,
			cfg.Generator.Winevt.Rate,
		)
		if err != nil {
			logger.Error("Failed to create winevt generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypePaloAlto:
		generatorInstance, err = paloalto.New(
			logger,
			cfg.Generator.PaloAlto.Workers,
			cfg.Generator.PaloAlto.Rate,
		)
		if err != nil {
			logger.Error("Failed to create palo-alto generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeApache:
		generatorInstance, err = apachegen.New(
			logger,
			cfg.Generator.Apache.Workers,
			cfg.Generator.Apache.Rate,
		)
		if err != nil {
			logger.Error("Failed to create Apache generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeApacheCombined:
		generatorInstance, err = apachecombinedgen.New(
			logger,
			cfg.Generator.ApacheCombined.Workers,
			cfg.Generator.ApacheCombined.Rate,
		)
		if err != nil {
			logger.Error("Failed to create Apache Combined generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeApacheError:
		generatorInstance, err = apacheerrorgen.New(
			logger,
			cfg.Generator.ApacheError.Workers,
			cfg.Generator.ApacheError.Rate,
		)
		if err != nil {
			logger.Error("Failed to create Apache Error generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeNginx:
		generatorInstance, err = nginx.New(
			logger,
			cfg.Generator.Nginx.Workers,
			cfg.Generator.Nginx.Rate,
		)
		if err != nil {
			logger.Error("Failed to create NGINX generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypePostgres:
		generatorInstance, err = postgres.New(
			logger,
			cfg.Generator.Postgres.Workers,
			cfg.Generator.Postgres.Rate,
		)
		if err != nil {
			logger.Error("Failed to create PostgreSQL generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeKubernetes:
		generatorInstance, err = kubernetes.New(
			logger,
			cfg.Generator.Kubernetes.Workers,
			cfg.Generator.Kubernetes.Rate,
			cfg.Generator.Kubernetes.Format,
		)
		if err != nil {
			logger.Error("Failed to create Kubernetes generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeFile:
		generatorInstance, err = filegen.New(
			logger,
			cfg.Generator.Filegen.Workers,
			cfg.Generator.Filegen.Rate,
			cfg.Generator.Filegen.Source,
			cfg.Generator.Filegen.CacheEnabled,
			cfg.Generator.Filegen.CacheTTL,
		)
		if err != nil {
			logger.Error("Failed to create File generator", zap.Error(err))
			return err
		}
	case config.GeneratorTypeOkta:
		generatorInstance, err = okta.New(
			logger,
			cfg.Generator.Okta.Workers,
			cfg.Generator.Okta.Rate,
		)
		if err != nil {
			logger.Error("Failed to create Okta generator", zap.Error(err))
			return err
		}
	default:
		logger.Error("Invalid generator type", zap.String("type", string(cfg.Generator.Type)))
		return fmt.Errorf("invalid generator type: %s", cfg.Generator.Type)
	}

	// Set up finite generation count tracker
	var tracker *count.Tracker
	if cfg.Generator.Count > 0 {
		tracker = count.NewTracker(int64(cfg.Generator.Count))
		if s, ok := generatorInstance.(interface{ SetCountTracker(*count.Tracker) }); ok {
			s.SetCountTracker(tracker)
		}
		logger.Info("Finite generation enabled",
			zap.Int("count", cfg.Generator.Count),
			zap.String("onFinish", cfg.OnFinish))
	}

	// Set up SIGUSR1 restart signal handler
	setupRestartSignal(ctx, logger, tracker)

	svc, err := service.New(logger, generatorInstance, outputInstance)
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
