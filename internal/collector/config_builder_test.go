package collector

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/require"
	"testing"
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

	exporterConfig := MakeExporterConfig(output, false)
	require.NotNil(t, exporterConfig)

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

	exporterConfig := MakeExporterConfig(output, false)
	require.NotNil(t, exporterConfig)

	require.Equal(t, 1, len(exporterConfig.OTLP.Headers))
	require.Equal(t, "${HEADER_AUTHORIZATION}", exporterConfig.OTLP.Headers["Authorization"])
}
