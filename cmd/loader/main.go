// Package main is the main package for the Bindplane Loader.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/observiq/bindplane-loader/generator"
	"github.com/observiq/bindplane-loader/internal/config"
	"github.com/observiq/bindplane-loader/internal/logging"
	"github.com/observiq/bindplane-loader/internal/service"
	"github.com/observiq/bindplane-loader/internal/telemetry/metrics"
	"github.com/observiq/bindplane-loader/output"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// Bind overrides to flags and environment variables
	flags := pflag.NewFlagSet("bindplane-loader", pflag.ExitOnError)

	// Add config file flag
	configFile := flags.String("config", "", "path to configuration file")

	for _, override := range config.DefaultOverrides() {
		if err := override.Bind(flags); err != nil {
			fmt.Printf("Failed to bind override %s: %s", override.Field, err.Error())
			os.Exit(1)
		}
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		fmt.Printf("Failed to parse flags: %s", err.Error())
		os.Exit(1)
	}

	// Configure Viper to handle env overrides
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read configuration file if provided
	if *configFile != "" {
		viper.SetConfigFile(*configFile)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Failed to read config file %s: %s", *configFile, err.Error())
			os.Exit(1)
		}
	}

	cfg := config.NewConfig()
	if err := viper.Unmarshal(cfg); err != nil {
		fmt.Printf("Failed to unmarshal config: %s", err.Error())
		os.Exit(1)
	}

	// Apply defaults for any empty fields
	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Failed to validate config: %s", err.Error())
		os.Exit(1)
	}

	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %s", err.Error())
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	logger.Info("bindplane-loader started")

	// Create signal context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup metrics
	if err := setupMetrics(ctx, logger); err != nil {
		logger.Error("Failed to setup metrics", zap.Error(err))
		os.Exit(1)
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
		outputInstance, err = output.NewNopOutput(logger)
		if err != nil {
			logger.Error("Failed to create NOP output", zap.Error(err))
			os.Exit(1)
		}
	case config.OutputTypeTCP:
		outputInstance, err = output.NewTCP(
			logger,
			cfg.Output.TCP.Host,
			strconv.Itoa(cfg.Output.TCP.Port),
			cfg.Output.TCP.Workers,
		)
		if err != nil {
			logger.Error("Failed to create TCP output", zap.Error(err))
			os.Exit(1)
		}
	case config.OutputTypeUDP:
		outputInstance, err = output.NewUDP(
			logger,
			cfg.Output.UDP.Host,
			strconv.Itoa(cfg.Output.UDP.Port),
			cfg.Output.UDP.Workers,
		)
		if err != nil {
			logger.Error("Failed to create UDP output", zap.Error(err))
			os.Exit(1)
		}
	default:
		logger.Error("Invalid output type", zap.String("type", string(cfg.Output.Type)))
		os.Exit(1)
	}

	// Configure generator
	var generatorInstance generator.Generator
	switch cfg.Generator.Type {
	case config.GeneratorTypeNop:
		generatorInstance, err = generator.NewNopGenerator(logger)
		if err != nil {
			logger.Error("Failed to create NOP generator", zap.Error(err))
			os.Exit(1)
		}
	case config.GeneratorTypeJSON:
		generatorInstance, err = generator.NewJSONGenerator(
			logger,
			cfg.Generator.JSON.Workers,
			cfg.Generator.JSON.Rate,
		)
		if err != nil {
			logger.Error("Failed to create JSON generator", zap.Error(err))
			os.Exit(1)
		}
	default:
		logger.Error("Invalid generator type", zap.String("type", string(cfg.Generator.Type)))
		os.Exit(1)
	}

	service, err := service.New(logger, generatorInstance, outputInstance)
	if err != nil {
		logger.Error("Failed to create service", zap.Error(err))
		os.Exit(1)
	}

	if err := service.Start(); err != nil {
		logger.Error("Failed to start service", zap.Error(err))
		os.Exit(1)
	}

	<-ctx.Done()

	if err := service.Stop(); err != nil {
		logger.Error("Failed to stop service", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("bindplane-loader shutdown complete")
}

func setupMetrics(ctx context.Context, logger *zap.Logger) error {
	logger.Info("starting metrics server")

	prometheus, err := metrics.NewPrometheus()
	if err != nil {
		return fmt.Errorf("new prometheus: %w", err)
	}

	if err := prometheus.Start(ctx); err != nil {
		return fmt.Errorf("start prometheus exporter: %w", err)
	}

	go func() {
		err := httpServer(logger)
		if err != nil {
			logger.Error("http server", zap.Error(err))
		}
	}()

	logger.Info("metrics server started")

	return nil
}

func httpServer(logger *zap.Logger) error {
	addr := net.JoinHostPort("0.0.0.0", "9100")

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
