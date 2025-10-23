// Package logging contains the logging logic for Blitz
package logging

import (
	"fmt"
	"os"
	"strings"

	loaderconfig "github.com/observiq/blitz/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger returns a new Logger for the specified config.
// If the config is empty, it defaults to stdout at info level.
func NewLogger(cfg loaderconfig.Logging) (*zap.Logger, error) {
	level := parseZapLevel(cfg.Level)

	// Only stdout supported for now. Default to stdout when empty.
	output := strings.TrimSpace(strings.ToLower(cfg.Type))
	if output == "" {
		output = loaderconfig.LoggingTypeStdout
	}
	if output != loaderconfig.LoggingTypeStdout {
		return nil, fmt.Errorf("unknown output type: %s", cfg.Type)
	}

	core := newStdoutCore(level)
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
