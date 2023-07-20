package otlpexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestGetOutputTypeHttp(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "http",
	}

	require.Equal(t, "otlphttp/test", ExporterID(output, "test"))
}

func TestGetOutputTypeOtlp(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "grpc",
	}

	require.Equal(t, "otlp/test", ExporterID(output, "test"))
}

func TestGetOutputTypeDefault(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	require.Equal(t, "otlp/test", ExporterID(output, "test"))
}

func TestMakeExporterConfig(t *testing.T) {
	output := &v1alpha1.OtlpOutput{
		Endpoint: v1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	exporterConfig, envVars, err := MakeExporterConfig(context.Background(), fake.NewClientBuilder().Build(), output, "test", 512)
	require.NoError(t, err)
	require.NotNil(t, exporterConfig)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TEST"], []byte("otlp-endpoint"))

	require.Contains(t, exporterConfig, "otlp/test")
	otlpExporterConfig := exporterConfig["otlp/test"]

	require.Equal(t, "${OTLP_ENDPOINT_TEST}", otlpExporterConfig.Endpoint)
	require.True(t, otlpExporterConfig.SendingQueue.Enabled)
	require.Equal(t, 512, otlpExporterConfig.SendingQueue.QueueSize)

	require.True(t, otlpExporterConfig.RetryOnFailure.Enabled)
	require.Equal(t, "5s", otlpExporterConfig.RetryOnFailure.InitialInterval)
	require.Equal(t, "30s", otlpExporterConfig.RetryOnFailure.MaxInterval)
	require.Equal(t, "300s", otlpExporterConfig.RetryOnFailure.MaxElapsedTime)

	require.Contains(t, exporterConfig, "logging/test")
	loggingExporterConfig := exporterConfig["logging/test"]

	require.Equal(t, "basic", loggingExporterConfig.Verbosity)
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

	exporterConfig, envVars, err := MakeExporterConfig(context.Background(), fake.NewClientBuilder().Build(), output, "test", 512)
	require.NoError(t, err)
	require.NotNil(t, exporterConfig)
	require.NotNil(t, envVars)

	require.Contains(t, exporterConfig, "otlp/test")
	otlpExporterConfig := exporterConfig["otlp/test"]
	require.Equal(t, 1, len(otlpExporterConfig.Headers))
	require.Equal(t, "${HEADER_TEST_AUTHORIZATION}", otlpExporterConfig.Headers["Authorization"])
}
