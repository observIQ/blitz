package config

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggingValidate(t *testing.T) {
	cases := []struct {
		name     string
		logging  Logging
		expected error
	}{
		{name: "empty-type-error", logging: Logging{}, expected: errInvalidLoggingType},
		{name: "stdout-ok", logging: Logging{Type: "stdout"}, expected: nil},
		{name: "file-without-path", logging: Logging{Type: "file"}, expected: fmt.Errorf("logging.file.path is required when logging.type is file")},
		{name: "file-with-path-ok", logging: Logging{Type: "file", File: LoggingFileConfig{Path: "/var/log/blitz.log"}}, expected: nil},
		{name: "invalid-type", logging: Logging{Type: "invalid"}, expected: errInvalidLoggingType},
		{name: "empty-level-ok", logging: Logging{Type: "stdout", Level: ""}, expected: nil},
		{name: "debug-ok", logging: Logging{Type: "stdout", Level: LogLevelDebug}, expected: nil},
		{name: "info-ok", logging: Logging{Type: "stdout", Level: LogLevelInfo}, expected: nil},
		{name: "warn-ok", logging: Logging{Type: "stdout", Level: LogLevelWarn}, expected: nil},
		{name: "error-ok", logging: Logging{Type: "stdout", Level: LogLevelError}, expected: nil},
		{name: "invalid-level", logging: Logging{Type: "stdout", Level: "verbose"}, expected: errInvalidLoggingLevel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.logging.Validate()
			if tc.expected == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.expected != nil {
				if strings.Contains(tc.expected.Error(), "logging.file.path") {
					require.Contains(t, err.Error(), "logging.file.path is required")
				} else if errors.Is(tc.expected, errInvalidLoggingType) {
					require.True(t, errors.Is(err, errInvalidLoggingType))
					if strings.Contains(tc.expected.Error(), "logging.type is required") {
						require.Contains(t, err.Error(), "logging.type is required")
					}
				} else {
					require.True(t, errors.Is(err, tc.expected))
				}
			}
		})
	}
}
