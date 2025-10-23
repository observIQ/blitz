package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggingValidate(t *testing.T) {
	cases := []struct {
		name     string
		logging  Logging
		expected error
	}{
		{name: "empty-ok", logging: Logging{}, expected: nil},
		{name: "stdout-ok", logging: Logging{Type: "stdout"}, expected: nil},
		{name: "invalid-type", logging: Logging{Type: "file"}, expected: errInvalidLoggingType},
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
			require.True(t, errors.Is(err, tc.expected))
		})
	}
}
