package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlowGeneratorConfigValidate(t *testing.T) {
	ok := FlowGeneratorConfig{Workers: 1, Rate: time.Second}
	require.NoError(t, ok.Validate())
	for _, s := range []string{"", "default", "wan-edge", "datacenter", "ddos-target"} {
		c := ok
		c.Scenario = s
		require.NoError(t, c.Validate(), "scenario %q", s)
	}
	for name, c := range map[string]FlowGeneratorConfig{
		"zero workers":     {Workers: 0, Rate: time.Second},
		"zero rate":        {Workers: 1, Rate: 0},
		"negative rate":    {Workers: 1, Rate: -time.Second},
		"unknown scenario": {Workers: 1, Rate: time.Second, Scenario: "lan"},
	} {
		require.Error(t, c.Validate(), name)
	}
}

// TestFlowOutputEmptyProtocolMeansDefault pins the contract the flow output
// relies on: an empty protocol validates, and it means DefaultFlowProtocol.
func TestFlowOutputEmptyProtocolMeansDefault(t *testing.T) {
	require.NoError(t, (&FlowOutputConfig{Host: "localhost", Port: 2055}).Validate())
	require.Equal(t, "netflow-v9", DefaultFlowProtocol)
	require.Error(t, (&FlowOutputConfig{Host: "localhost", Port: 70000}).Validate())
	require.ErrorContains(t, (&FlowOutputConfig{Host: "localhost", Port: 2055, Protocol: "netflow-v7"}).Validate(), "protocol must be one of")
	require.ErrorContains(t, (&FlowOutputConfig{Host: "localhost", Port: 2055, Protocol: "netflow-v9", Vendor: "appflow"}).Validate(), `does not support protocol "netflow-v9"`)
	// An empty protocol is checked against the vendor as the default.
	require.ErrorContains(t, (&FlowOutputConfig{Host: "localhost", Port: 2055, Vendor: "appflow"}).Validate(), `does not support protocol "netflow-v9"`)
}

// TestFlowOutputRequiresAHost pins that a flow output names its collector: an
// empty host would otherwise dial ":port", the local machine.
func TestFlowOutputRequiresAHost(t *testing.T) {
	require.ErrorContains(t, (&FlowOutputConfig{Port: 2055}).Validate(), "host cannot be empty")
}
