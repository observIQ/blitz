// Package logging contains the logging logic for Blitz
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loaderconfig "github.com/observiq/blitz/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger returns a new Logger for the specified config.
// If the config is empty, it defaults to file at info level.
func NewLogger(cfg loaderconfig.Logging) (*zap.Logger, error) {
	level := parseZapLevel(cfg.Level)

	// Default to file when empty.
	output := strings.TrimSpace(strings.ToLower(cfg.Type))
	if output == "" {
		output = loaderconfig.LoggingTypeFile
	}

	var core zapcore.Core
	switch output {
	case loaderconfig.LoggingTypeStdout:
		core = newStdoutCore(level)
	case loaderconfig.LoggingTypeFile:
		fileCore, err := newFileCore(level, cfg.File)
		if err != nil {
			return nil, fmt.Errorf("create file core: %w", err)
		}
		core = fileCore
	default:
		return nil, fmt.Errorf("unknown output type: %s", cfg.Type)
	}

	return zap.New(core), nil
}

func parseZapLevel(level loaderconfig.LogLevel) zapcore.Level {
	switch strings.ToLower(string(level)) {
	case string(loaderconfig.LogLevelDebug):
		return zapcore.DebugLevel
	case string(loaderconfig.LogLevelWarn):
		return zapcore.WarnLevel
	case string(loaderconfig.LogLevelError):
		return zapcore.ErrorLevel
	case string(loaderconfig.LogLevelInfo):
		fallthrough
	case "":
		return zapcore.InfoLevel
	default:
		return zapcore.InfoLevel
	}
}

func newStdoutCore(level zapcore.Level) zapcore.Core {
	return zapcore.NewCore(newEncoder(), zapcore.Lock(os.Stdout), level)
}

func newEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.CallerKey = ""
	encoderConfig.StacktraceKey = ""
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.MessageKey = "message"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func newFileCore(level zapcore.Level, cfg loaderconfig.LoggingFileConfig) (zapcore.Core, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = loaderconfig.DefaultLoggingFilePath
	}

	// Verify directory exists (should be created by package or user)
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log directory %s does not exist", dir)
		}
		return nil, fmt.Errorf("check log directory: %w", err)
	}

	// Set up rotation defaults if not provided
	maxSizeMB := cfg.Rotation.MaxSizeMB
	if maxSizeMB == 0 {
		maxSizeMB = loaderconfig.DefaultFileRotationMaxSizeMB
	}
	maxBackups := cfg.Rotation.MaxBackups
	if maxBackups == 0 {
		maxBackups = loaderconfig.DefaultFileRotationMaxBackups
	}
	maxAgeDays := cfg.Rotation.MaxAgeDays
	if maxAgeDays == 0 {
		maxAgeDays = loaderconfig.DefaultFileRotationMaxAgeDays
	}

	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   cfg.Rotation.Compress,
		LocalTime:  cfg.Rotation.LocalTime,
	}

	return zapcore.NewCore(newEncoder(), zapcore.AddSync(writer), level), nil
}
