package builder

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type EnvVars map[string][]byte

func getOTLPOutputAlias(output *telemetryv1alpha1.OtlpOutput, pipelineName string) string {
	var outputType string
	if output.Protocol == "http" {
		outputType = "otlphttp"
	} else {
		outputType = "otlp"
	}

	return fmt.Sprintf("%s/%s", outputType, pipelineName)
}

func getLoggingOutputAlias(pipelineName string) string {
	return fmt.Sprintf("logging/%s", pipelineName)
}

func GetExporterAliases(exporters map[string]any) []string {
	var aliases []string
	for alias := range exporters {
		aliases = append(aliases, alias)
	}
	return aliases
}

func MakeOTLPExporterConfig(ctx context.Context, c client.Reader, otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string) (map[string]any, EnvVars, error) {
	envVars, err := makeEnvVars(ctx, c, otlpOutput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %v", err)
	}

	config := makeExporterConfig(otlpOutput, pipelineName, envVars)
	return config, envVars, nil
}

func makeExporterConfig(otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string, secretData map[string][]byte) map[string]any {
	otlpOutputAlias := getOTLPOutputAlias(otlpOutput, pipelineName)
	loggingOutputAlias := getLoggingOutputAlias(pipelineName)
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

	return map[string]any{
		otlpOutputAlias:    otlpExporterConfig,
		loggingOutputAlias: loggingExporter,
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
