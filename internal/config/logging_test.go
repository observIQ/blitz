package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggingValidate(t *testing.T) {
	cases := []struct {
		name           string
		logging        Logging
		expectedErrMsg string
	}{
		{name: "empty-type-error", logging: Logging{}, expectedErrMsg: "logging.type is required"},
		{name: "stdout-ok", logging: Logging{Type: "stdout"}, expectedErrMsg: ""},
		{name: "file-without-path", logging: Logging{Type: "file"}, expectedErrMsg: "logging.file.path is required"},
		{name: "file-with-path-ok", logging: Logging{Type: "file", File: LoggingFileConfig{Path: "/var/log/blitz.log"}}, expectedErrMsg: ""},
		{name: "invalid-type", logging: Logging{Type: "invalid"}, expectedErrMsg: "invalid logging type"},
		{name: "empty-level-ok", logging: Logging{Type: "stdout", Level: ""}, expectedErrMsg: ""},
		{name: "debug-ok", logging: Logging{Type: "stdout", Level: LogLevelDebug}, expectedErrMsg: ""},
		{name: "info-ok", logging: Logging{Type: "stdout", Level: LogLevelInfo}, expectedErrMsg: ""},
		{name: "warn-ok", logging: Logging{Type: "stdout", Level: LogLevelWarn}, expectedErrMsg: ""},
		{name: "error-ok", logging: Logging{Type: "stdout", Level: LogLevelError}, expectedErrMsg: ""},
		{name: "invalid-level", logging: Logging{Type: "stdout", Level: "verbose"}, expectedErrMsg: "invalid logging level"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.logging.Validate()
			if tc.expectedErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedErrMsg)
			}
		})
	}
}
