// Package main is the main package for the Bindplane Loader.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/observiq/bindplane-loader/internal/config"
	"github.com/observiq/bindplane-loader/internal/logging"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
}
