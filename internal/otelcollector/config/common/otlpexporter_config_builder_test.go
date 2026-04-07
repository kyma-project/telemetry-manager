package common

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func traceRefTest() PipelineRef {
	return TracePipelineRef(&telemetryv1beta1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
}

func metricRefTest() PipelineRef {
	return MetricPipelineRef(&telemetryv1beta1.MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
}

func TestExporterIDHTTP(t *testing.T) {
	require.Equal(t, "otlp_http/tracepipeline-test", OTLPExporterID("http", traceRefTest()))
}

func TestExporterIDGRPC(t *testing.T) {
	require.Equal(t, "otlp_grpc/tracepipeline-test", OTLPExporterID("grpc", traceRefTest()))
}

func TestExorterIDDefault(t *testing.T) {
	require.Equal(t, "otlp_grpc/tracepipeline-test", OTLPExporterID("", traceRefTest()))
}

func TestMakeExporterConfig(t *testing.T) {
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TRACEPIPELINE_TEST"], []byte("otlp-endpoint"))

	require.Equal(t, "${OTLP_ENDPOINT_TRACEPIPELINE_TEST}", otlpExporterConfig.Endpoint)
	require.True(t, otlpExporterConfig.SendingQueue.Enabled)
	require.Equal(t, 512, otlpExporterConfig.SendingQueue.QueueSize)

	require.True(t, otlpExporterConfig.RetryOnFailure.Enabled)
	require.Equal(t, "5s", otlpExporterConfig.RetryOnFailure.InitialInterval)
	require.Equal(t, "30s", otlpExporterConfig.RetryOnFailure.MaxInterval)
	require.Equal(t, "300s", otlpExporterConfig.RetryOnFailure.MaxElapsedTime)
}

func TestMakeExporterConfigTraceWithPath(t *testing.T) {
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		Path:     "/v1/test",
		Protocol: "http",
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_TRACEPIPELINE_TEST"], []byte("otlp-endpoint/v1/test"))

	require.Equal(t, "${OTLP_ENDPOINT_TRACEPIPELINE_TEST}", otlpExporterConfig.TracesEndpoint)
	require.Empty(t, otlpExporterConfig.Endpoint)
}

func TestMakeExporterConfigMetricWithPath(t *testing.T) {
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		Path:     "/v1/test",
		Protocol: "http",
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, metricRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OTLP_ENDPOINT_METRICPIPELINE_TEST"])
	require.Equal(t, envVars["OTLP_ENDPOINT_METRICPIPELINE_TEST"], []byte("otlp-endpoint/v1/test"))

	require.Equal(t, "${OTLP_ENDPOINT_METRICPIPELINE_TEST}", otlpExporterConfig.MetricsEndpoint)
	require.Empty(t, otlpExporterConfig.Endpoint)
}

func TestMakeExporterConfigWithBasicAuth(t *testing.T) {
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		Authentication: &telemetryv1beta1.AuthenticationOptions{
			Basic: &telemetryv1beta1.BasicAuthOptions{
				User:     telemetryv1beta1.ValueType{Value: "testuser"},
				Password: telemetryv1beta1.ValueType{Value: "testpass"},
			},
		},
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.Equal(t, 1, len(otlpExporterConfig.Headers))
	require.Equal(t, "${BASIC_AUTH_HEADER_TRACEPIPELINE_TEST}", otlpExporterConfig.Headers["Authorization"])

	require.NotNil(t, envVars["BASIC_AUTH_HEADER_TRACEPIPELINE_TEST"])

	base64UserPass := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
	require.Equal(t, envVars["BASIC_AUTH_HEADER_TRACEPIPELINE_TEST"], []byte("Basic "+base64UserPass))
}

func TestMakeExporterConfigWithOAuth2(t *testing.T) {
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		Authentication: &telemetryv1beta1.AuthenticationOptions{
			OAuth2: &telemetryv1beta1.OAuth2Options{
				TokenURL:     telemetryv1beta1.ValueType{Value: "token-url"},
				ClientID:     telemetryv1beta1.ValueType{Value: "client-id"},
				ClientSecret: telemetryv1beta1.ValueType{Value: "client-secret"},
			},
		},
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, otlpExporterConfig.Auth)
	require.Equal(t, "oauth2client/tracepipeline-test", otlpExporterConfig.Auth.Authenticator)
}

func TestMakeExporterConfigWithCustomHeaders(t *testing.T) {
	headers := []telemetryv1beta1.Header{
		{
			Name: "Authorization",
			ValueType: telemetryv1beta1.ValueType{
				Value: "Bearer xyz",
			},
		},
	}
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		Headers:  headers,
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.Equal(t, 1, len(otlpExporterConfig.Headers))
	require.Equal(t, "${HEADER_TRACEPIPELINE_TEST_AUTHORIZATION}", otlpExporterConfig.Headers["Authorization"])
}

func TestMakeExporterConfigWithTLSInsecure(t *testing.T) {
	tls := &telemetryv1beta1.OutputTLS{
		Insecure: true,
	}
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.True(t, otlpExporterConfig.TLS.Insecure)
}

func TestMakeExporterConfigWithTLSInsecureSkipVerify(t *testing.T) {
	tls := &telemetryv1beta1.OutputTLS{
		Insecure:           false,
		InsecureSkipVerify: true,
	}
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.False(t, otlpExporterConfig.TLS.Insecure)
	require.True(t, otlpExporterConfig.TLS.InsecureSkipVerify)
	require.Nil(t, envVars["TLS_CONFIG_CA_TRACEPIPELINE_TEST"])
}

func TestMakeExporterConfigWithmTLS(t *testing.T) {
	tls := &telemetryv1beta1.OutputTLS{
		Insecure:           false,
		InsecureSkipVerify: false,
		CA: &telemetryv1beta1.ValueType{
			Value: "test ca cert pem",
		},
		Cert: &telemetryv1beta1.ValueType{
			Value: "test client cert pem",
		},
		Key: &telemetryv1beta1.ValueType{
			Value: "test client key pem",
		},
	}
	output := &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
		TLS:      tls,
	}

	cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
	otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.False(t, otlpExporterConfig.TLS.Insecure)
	require.False(t, otlpExporterConfig.TLS.InsecureSkipVerify)
	require.Equal(t, "${OTLP_TLS_CA_PEM_TRACEPIPELINE_TEST}", otlpExporterConfig.TLS.CAPem)
	require.Equal(t, "${OTLP_TLS_CERT_PEM_TRACEPIPELINE_TEST}", otlpExporterConfig.TLS.CertPem)
	require.Equal(t, "${OTLP_TLS_KEY_PEM_TRACEPIPELINE_TEST}", otlpExporterConfig.TLS.KeyPem)

	require.NotNil(t, envVars["OTLP_TLS_CA_PEM_TRACEPIPELINE_TEST"])
	require.NotNil(t, envVars["OTLP_TLS_CERT_PEM_TRACEPIPELINE_TEST"])
	require.NotNil(t, envVars["OTLP_TLS_KEY_PEM_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OTLP_TLS_CA_PEM_TRACEPIPELINE_TEST"], []byte("test ca cert pem"))
	require.Equal(t, envVars["OTLP_TLS_CERT_PEM_TRACEPIPELINE_TEST"], []byte("test client cert pem"))
	require.Equal(t, envVars["OTLP_TLS_KEY_PEM_TRACEPIPELINE_TEST"], []byte("test client key pem"))
}

func TestMakeExporterConfigCompression(t *testing.T) {
	tests := []struct {
		name                string
		compression         telemetryv1beta1.OTLPCompressionEncoding
		expectedCompression string
	}{
		{
			name:                "snappy compression",
			compression:         telemetryv1beta1.OTLPCompressionSnappy,
			expectedCompression: "snappy",
		},
		{
			name:                "no compression set defaults to gzip",
			compression:         "",
			expectedCompression: "gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &telemetryv1beta1.OTLPOutput{
				Endpoint:    telemetryv1beta1.ValueType{Value: "otlp-endpoint"},
				Compression: tt.compression,
			}

			cb := NewOTLPExporterConfigBuilder(fake.NewClientBuilder().Build(), output, traceRefTest(), 512)
			otlpExporterConfig, envVars, err := cb.OTLPExporter(t.Context())
			require.NoError(t, err)
			require.NotNil(t, envVars)

			require.Equal(t, tt.expectedCompression, otlpExporterConfig.Compression)
		})
	}
}
