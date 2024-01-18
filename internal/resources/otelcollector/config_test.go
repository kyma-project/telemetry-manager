package otelcollector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayConfig(t *testing.T) {
	gatewayScalingConfig := GatewayScalingConfig{
		Replicas:                       2,
		ResourceRequirementsMultiplier: 2,
	}
	collectorCfgYAML := "test yaml"
	collectorEnvVars := map[string][]byte{
		"test-key": {byte('a')},
	}
	excludePorts := "1111"
	istioEnabled := true
	allowedPorts := []int32{8000, 8080}

	gatewayConfig := &GatewayConfig{}
	gatewayConfig = gatewayConfig.
		WithScaling(gatewayScalingConfig).
		WithCollectorConfig(collectorCfgYAML, collectorEnvVars).
		WithIstioConfig(excludePorts, istioEnabled).
		WithAllowedPorts(allowedPorts)

	require.Equal(t, gatewayScalingConfig, gatewayConfig.Scaling)
	require.Equal(t, collectorCfgYAML, gatewayConfig.CollectorConfig)
	require.Equal(t, istioEnabled, gatewayConfig.Istio.Enabled)
	require.Equal(t, excludePorts, gatewayConfig.Istio.ExcludePorts)
	require.Equal(t, allowedPorts, gatewayConfig.allowedPorts)
}

func TestAgentConfig(t *testing.T) {
	collectorCfgYAML := "test yaml"
	allowedPorts := []int32{8000, 8080}

	agentConfig := &AgentConfig{}
	agentConfig = agentConfig.
		WithCollectorConfig(collectorCfgYAML).
		WithAllowedPorts(allowedPorts)

	require.Equal(t, collectorCfgYAML, agentConfig.CollectorConfig)
	require.Equal(t, allowedPorts, agentConfig.allowedPorts)
}
