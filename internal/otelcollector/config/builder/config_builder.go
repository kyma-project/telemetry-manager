package builder

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

func GetOutputType(output *telemetryv1alpha1.OtlpOutput) string {
	if output.Protocol == "http" {
		return "otlphttp"
	}
	return "otlp"
}

func makeHeaders(output *telemetryv1alpha1.OtlpOutput) map[string]string {
	headers := make(map[string]string)
	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		headers["Authorization"] = "${BASIC_AUTH_HEADER}"
	}

	for _, header := range output.Headers {
		headers[header.Name] = fmt.Sprintf("${HEADER_%s}", envvar.MakeEnvVarCompliant(header.Name))
	}
	return headers
}

func MakeExporterConfig(output *telemetryv1alpha1.OtlpOutput, insecureOutput bool) config.ExporterConfig {
	outputType := GetOutputType(output)
	headers := makeHeaders(output)
	otlpExporterConfig := config.OTLPExporterConfig{
		Endpoint: fmt.Sprintf("${%s}", "OTLP_ENDPOINT"),
		Headers:  headers,
		TLS: config.TLSConfig{
			Insecure: insecureOutput,
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

func MakeExtensionConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:13133",
		},
	}
}
