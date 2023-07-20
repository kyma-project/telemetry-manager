package otlpexporter

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

type EnvVars map[string][]byte

func MakeExportersConfig(ctx context.Context, c client.Reader, otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string, queueSize int) (map[string]common.BaseGatewayExporterConfig, EnvVars, error) {
	envVars, err := makeEnvVars(ctx, c, otlpOutput, pipelineName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %v", err)
	}

	exportersConfig := makeExportersConfig(otlpOutput, pipelineName, envVars, queueSize)
	return exportersConfig, envVars, nil
}

func makeExportersConfig(otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string, secretData map[string][]byte, queueSize int) map[string]common.BaseGatewayExporterConfig {
	otlpOutputAlias := getOTLPOutputAlias(otlpOutput, pipelineName)
	loggingOutputAlias := getLoggingOutputAlias(pipelineName)
	headers := makeHeaders(otlpOutput, pipelineName)
	otlpEndpointVariable := makeOtlpEndpointVariable(pipelineName)
	otlpExporterConfig := common.OTLPExporterConfig{
		Endpoint: fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:  headers,
		TLS: common.TLSConfig{
			Insecure: isInsecureOutput(string(secretData[otlpEndpointVariable])),
		},
		SendingQueue: common.SendingQueueConfig{
			Enabled:   true,
			QueueSize: queueSize,
		},
		RetryOnFailure: common.RetryOnFailureConfig{
			Enabled:         true,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			MaxElapsedTime:  "300s",
		},
	}

	loggingExporter := common.LoggingExporterConfig{
		Verbosity: "basic",
	}

	return map[string]common.BaseGatewayExporterConfig{
		otlpOutputAlias:    {OTLPExporterConfig: &otlpExporterConfig},
		loggingOutputAlias: {LoggingExporterConfig: &loggingExporter},
	}
}

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

func makeHeaders(output *telemetryv1alpha1.OtlpOutput, pipelineName string) map[string]string {
	headers := make(map[string]string)
	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		basicAuthHeaderVariable := makeBasicAuthHeaderVariable(pipelineName)
		headers["Authorization"] = fmt.Sprintf("${%s}", basicAuthHeaderVariable)
	}

	for _, header := range output.Headers {
		headers[header.Name] = fmt.Sprintf("${%s}", makeHeaderEnvVarCompliant(header, pipelineName))
	}
	return headers
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}
