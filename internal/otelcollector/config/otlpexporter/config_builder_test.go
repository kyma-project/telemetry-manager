package otlpexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestExporterIDHTTP(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "http",
	}

	require.Equal(t, "otlphttp/test", ExporterID(output, "test"))
}

func TestExporterIDGRPC(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		Protocol: "grpc",
	}

	require.Equal(t, "otlp/test", ExporterID(output, "test"))
}

func TestExorterIDDefault(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	require.Equal(t, "otlp/test", ExporterID(output, "test"))
}

func TestMakeConfig(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TEST"], []byte("otlp-endpoint"))

	require.Equal(t, "${OTLP_ENDPOINT_TEST}", otlpExporterConfig.Endpoint)
	require.True(t, otlpExporterConfig.SendingQueue.Enabled)
	require.Equal(t, 512, otlpExporterConfig.SendingQueue.QueueSize)

	require.True(t, otlpExporterConfig.RetryOnFailure.Enabled)
	require.Equal(t, "5s", otlpExporterConfig.RetryOnFailure.InitialInterval)
	require.Equal(t, "30s", otlpExporterConfig.RetryOnFailure.MaxInterval)
	require.Equal(t, "300s", otlpExporterConfig.RetryOnFailure.MaxElapsedTime)
}

func TestMakeConfigTraceWithPath(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		Path:     "/v1/test",
		Protocol: "http",
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TEST"], []byte("otlp-endpoint/v1/test"))

	require.Equal(t, "${OTLP_ENDPOINT_TEST}", otlpExporterConfig.TracesEndpoint)
	require.Empty(t, otlpExporterConfig.Endpoint)
}

func TestMakeConfigMetricWithPath(t *testing.T) {
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		Path:     "/v1/test",
		Protocol: "http",
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeMetric)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TEST"], []byte("otlp-endpoint/v1/test"))

	require.Equal(t, "${OTLP_ENDPOINT_TEST}", otlpExporterConfig.MetricsEndpoint)
	require.Empty(t, otlpExporterConfig.Endpoint)
}

func TestMakeExporterConfigWithCustomHeaders(t *testing.T) {
	headers := []telemetryv1alpha1.Header{
		{
			Name: "Authorization",
			ValueType: telemetryv1alpha1.ValueType{
				Value: "Bearer xyz",
			},
		},
	}
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		Headers:  headers,
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.Equal(t, 1, len(otlpExporterConfig.Headers))
	require.Equal(t, "${HEADER_TEST_AUTHORIZATION}", otlpExporterConfig.Headers["Authorization"])
}

func TestMakeExporterConfigWithTLSInsecure(t *testing.T) {
	tls := &telemetryv1alpha1.OtlpTLS{
		Insecure: true,
	}
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.True(t, otlpExporterConfig.TLS.Insecure)
}

func TestMakeExporterConfigWithTLSInsecureSkipVerify(t *testing.T) {
	tls := &telemetryv1alpha1.OtlpTLS{
		Insecure:           false,
		InsecureSkipVerify: true,
	}
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.False(t, otlpExporterConfig.TLS.Insecure)
	require.True(t, otlpExporterConfig.TLS.InsecureSkipVerify)
	require.Nil(t, envVars["TLS_CONFIG_CA_TEST"])
}

func TestMakeExporterConfigWithmTLS(t *testing.T) {
	tls := &telemetryv1alpha1.OtlpTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: &telemetryv1alpha1.ValueType{
			Value: "test ca cert pem",
		},
		Cert: &telemetryv1alpha1.ValueType{
			Value: "test client cert pem",
		},
		Key: &telemetryv1alpha1.ValueType{
			Value: "test client key pem",
		},
	}
	output := &telemetryv1alpha1.OtlpOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewConfigBuilder(fake.NewClientBuilder().Build(), output, "test", 512, SignalTypeTrace)
	otlpExporterConfig, envVars, err := cb.MakeConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.False(t, otlpExporterConfig.TLS.Insecure)
	require.False(t, otlpExporterConfig.TLS.InsecureSkipVerify)
	require.Equal(t, "${OTLP_TLS_CA_PEM_TEST}", otlpExporterConfig.TLS.CAPem)
	require.Equal(t, "${OTLP_TLS_CERT_PEM_TEST}", otlpExporterConfig.TLS.CertPem)
	require.Equal(t, "${OTLP_TLS_KEY_PEM_TEST}", otlpExporterConfig.TLS.KeyPem)

	require.NotNil(t, envVars["OTLP_TLS_CA_PEM_TEST"])
	require.NotNil(t, envVars["OTLP_TLS_CERT_PEM_TEST"])
	require.NotNil(t, envVars["OTLP_TLS_KEY_PEM_TEST"])
	require.Equal(t, envVars["OTLP_TLS_CA_PEM_TEST"], []byte("test ca cert pem"))
	require.Equal(t, envVars["OTLP_TLS_CERT_PEM_TEST"], []byte("test client cert pem"))
	require.Equal(t, envVars["OTLP_TLS_KEY_PEM_TEST"], []byte("test client key pem"))

}
