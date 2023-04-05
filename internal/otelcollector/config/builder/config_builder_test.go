package builder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func TestGetOutputTypeHttp(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "http",
	}

	require.Equal(t, "otlphttp", GetOutputType(output))
}

func TestGetOutputTypeOtlp(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "grpc",
	}

	require.Equal(t, "otlp", GetOutputType(output))
}

func TestGetOutputTypeDefault(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	require.Equal(t, "otlp", GetOutputType(output))
}

func TestMakeExporterConfig(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	exporterConfig, envVars, err := MakeOTLPExporterConfig(context.Background(), fake.NewClientBuilder().Build(), output)
	require.NoError(t, err)
	require.NotNil(t, exporterConfig)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT"])
	require.Equal(t, envVars["OTLP_ENDPOINT"], []byte("otlp-endpoint"))

	require.Equal(t, "${OTLP_ENDPOINT}", exporterConfig.OTLP.Endpoint)
	require.True(t, exporterConfig.OTLP.SendingQueue.Enabled)
	require.Equal(t, 512, exporterConfig.OTLP.SendingQueue.QueueSize)

	require.True(t, exporterConfig.OTLP.RetryOnFailure.Enabled)
	require.Equal(t, "5s", exporterConfig.OTLP.RetryOnFailure.InitialInterval)
	require.Equal(t, "30s", exporterConfig.OTLP.RetryOnFailure.MaxInterval)
	require.Equal(t, "300s", exporterConfig.OTLP.RetryOnFailure.MaxElapsedTime)

	require.Equal(t, "basic", exporterConfig.Logging.Verbosity)
}

func TestMakeExporterConfigWithCustomHeaders(t *testing.T) {
	headers := []v1alpha1.Header{
		{
			Name: "Authorization",
			ValueType: v1alpha1.ValueType{
				Value: "Bearer xyz",
			},
		},
	}
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
		Headers:  headers,
	}

	exporterConfig, envVars, err := MakeOTLPExporterConfig(context.Background(), fake.NewClientBuilder().Build(), output)
	require.NoError(t, err)
	require.NotNil(t, exporterConfig)
	require.NotNil(t, envVars)

	require.Equal(t, 1, len(exporterConfig.OTLP.Headers))
	require.Equal(t, "${HEADER_AUTHORIZATION}", exporterConfig.OTLP.Headers["Authorization"])
}

func TestMakeExtensionConfig(t *testing.T) {
	expectedConfig := config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:13133",
		},
	}

	actualConfig := MakeExtensionConfig()
	require.Equal(t, expectedConfig, actualConfig)
}
