// Package main is the main package for the Bindplane Loader.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/observiq/bindplane-loader/generator"
	"github.com/observiq/bindplane-loader/internal/config"
	"github.com/observiq/bindplane-loader/internal/logging"
	"github.com/observiq/bindplane-loader/internal/service"
	"github.com/observiq/bindplane-loader/output"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// Bind overrides to flags and environment variables
	flags := pflag.NewFlagSet("bindplane-loader", pflag.ExitOnError)
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

	cfg := config.NewConfig()
	if err := viper.Unmarshal(cfg); err != nil {
		fmt.Printf("Failed to unmarshal config: %s", err.Error())
		os.Exit(1)
	}

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
