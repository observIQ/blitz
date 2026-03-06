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
	"reflect"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/observiq/blitz/generator"
	apachegen "github.com/observiq/blitz/generator/apache"
	apachecombinedgen "github.com/observiq/blitz/generator/apache_combined"
	apacheerrorgen "github.com/observiq/blitz/generator/apache_error"
	"github.com/observiq/blitz/generator/filegen"
	jsongen "github.com/observiq/blitz/generator/json"
	"github.com/observiq/blitz/generator/kubernetes"
	metricsgen "github.com/observiq/blitz/generator/metrics"
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
	if err := viper.Unmarshal(cfg, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			flattenNestedMapHook(),
		),
	)); err != nil {
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
	case config.GeneratorTypeMetrics:
		metricDefs := make([]metricsgen.MetricDefinition, 0, len(cfg.Generator.Metrics.Metrics))
		for _, m := range cfg.Generator.Metrics.Metrics {
			metricDefs = append(metricDefs, metricsgen.MetricDefinition{
				Name:        m.Name,
				Type:        output.MetricType(m.Type),
				Description: m.Description,
				Unit:        m.Unit,
				Attributes:  m.Attributes,
				ValueMin:    m.ValueMin,
				ValueMax:    m.ValueMax,
			})
		}
		generatorInstance, err = metricsgen.New(
			logger,
			cfg.Generator.Metrics.Workers,
			cfg.Generator.Metrics.Rate,
			cfg.Generator.Metrics.ResourceAttributes,
			metricDefs,
		)

		if err != nil {
			logger.Error("Failed to create metrics generator", zap.Error(err))
			return err
		}
	default:
		logger.Error("Invalid generator type", zap.String("type", string(cfg.Generator.Type)))
		return fmt.Errorf("invalid generator type: %s", cfg.Generator.Type)
	}

	svc, err := service.New(logger, generatorInstance, outputInstance)
	if err != nil {
		logger.Error("Failed to create service", zap.Error(err))
		return err
	}

	if err := svc.Start(); err != nil {
		logger.Error("Failed to start service", zap.Error(err))
		return err
	}

	<-ctx.Done()

	if err := svc.Stop(); err != nil {
		logger.Error("Failed to stop service", zap.Error(err))
		return err
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

// flattenNestedMapHook returns a mapstructure DecodeHookFunc that flattens
// nested map[string]interface{} values into dotted-key maps. This is needed
// because Viper splits YAML keys containing "." into nested maps
// (e.g. "service.name: foo" becomes {"service": {"name": "foo"}}).
// Supports both map[string]string and map[string][]string target types.
func flattenNestedMapHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		src, ok := data.(map[string]interface{})
		if !ok {
			return data, nil
		}

		switch {
		case to == reflect.TypeOf(map[string]string{}):
			result := make(map[string]string)
			flattenMapString("", src, result)
			return result, nil

		case to == reflect.TypeOf(map[string][]string{}):
			result := make(map[string][]string)
			flattenMapSlice("", src, result)
			return result, nil

		default:
			return data, nil
		}
	}
}

func flattenMapString(prefix string, m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMapString(key, val, out)
		default:
			out[key] = fmt.Sprintf("%v", val)
		}
	}
}

func flattenMapSlice(prefix string, m map[string]interface{}, out map[string][]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMapSlice(key, val, out)
		case []interface{}:
			strs := make([]string, 0, len(val))
			for _, elem := range val {
				strs = append(strs, fmt.Sprintf("%v", elem))
			}
			out[key] = strs
		default:
			out[key] = []string{fmt.Sprintf("%v", val)}
		}
	}
}
