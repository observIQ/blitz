package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateExclusiveGeneratorConfig_BothSetFails(t *testing.T) {
	v := viperFrom(t, "generator:\n  type: json\ngenerators:\n  - type: json\n")
	require.Error(t, ValidateExclusiveGeneratorConfig(v, nil))
}

func TestValidateExclusiveGeneratorConfig_OnlyGeneratorsOK(t *testing.T) {
	v := viperFrom(t, "generators:\n  - type: json\n")
	require.NoError(t, ValidateExclusiveGeneratorConfig(v, nil))
}

func TestValidateExclusiveGeneratorConfig_OnlyGeneratorOK(t *testing.T) {
	v := viperFrom(t, "generator:\n  type: json\n")
	require.NoError(t, ValidateExclusiveGeneratorConfig(v, nil))
}

func TestValidateExclusiveGeneratorConfig_RejectsBadListForms(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"not a list":    {"generators: json\n", "generators must be a list of generator entries"},
		"empty list":    {"generators: []\n", "generators must not be empty"},
		"entry no type": {"generators:\n  - json:\n      workers: 2\n", "generators[0]: type is required"},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateExclusiveGeneratorConfig(viperFrom(t, tc.yaml), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}
