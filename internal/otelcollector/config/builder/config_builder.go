package builder

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type EnvVars map[string][]byte

func GetOutputType(output *telemetryv1alpha1.OtlpOutput) string {
	if output.Protocol == "http" {
		return "otlphttp"
	}
	return "otlp"
}

func MakeOTLPExporterConfig(ctx context.Context, c client.Reader, otlpOutput *telemetryv1alpha1.OtlpOutput) (*config.ExporterConfig, EnvVars, error) {
	envVars, err := makeEnvVars(ctx, c, otlpOutput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %v", err)
	}

	config := makeExporterConfig(otlpOutput, envVars)
	return &config, envVars, nil
}

func makeExporterConfig(otlpOutput *telemetryv1alpha1.OtlpOutput, secretData map[string][]byte) config.ExporterConfig {
	outputType := GetOutputType(otlpOutput)
	headers := makeHeaders(otlpOutput)
	otlpExporterConfig := config.OTLPExporterConfig{
		Endpoint: fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:  headers,
		TLS: config.TLSConfig{
			Insecure: isInsecureOutput(string(secretData[otlpEndpointVariable])),
		},
		SendingQueue: config.SendingQueueConfig{
			Enabled:   true,
			QueueSize: 512,
		},
		RetryOnFailure: config.RetryOnFailureConfig{
			Enabled:         true,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			MaxElapsedTime:  "300s",
		},
	}

	loggingExporter := config.LoggingExporterConfig{
		Verbosity: "basic",
	}

	if outputType == "otlphttp" {
		return config.ExporterConfig{
			OTLPHTTP: otlpExporterConfig,
			Logging:  loggingExporter,
		}
	}
	return config.ExporterConfig{
		OTLP:    otlpExporterConfig,
		Logging: loggingExporter,
	}
}

func makeHeaders(output *telemetryv1alpha1.OtlpOutput) map[string]string {
	headers := make(map[string]string)
	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		headers["Authorization"] = fmt.Sprintf("${%s}", basicAuthHeaderVariable)
	}

	for _, header := range output.Headers {
		headers[header.Name] = makeHeaderEnvVarCompliant(header)
	}
	return headers
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}

func MakeExtensionConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:13133",
		},
	}
}
