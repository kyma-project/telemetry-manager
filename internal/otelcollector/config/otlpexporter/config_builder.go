package otlpexporter

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type EnvVars map[string][]byte

type ConfigBuilder struct {
	reader       client.Reader
	otlpOutput   *telemetryv1alpha1.OtlpOutput
	pipelineName string
	queueSize    int
}

func NewConfigBuilder(reader client.Reader, otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string, queueSize int) *ConfigBuilder {
	return &ConfigBuilder{
		reader:       reader,
		otlpOutput:   otlpOutput,
		pipelineName: pipelineName,
		queueSize:    queueSize,
	}
}

func (cb *ConfigBuilder) MakeConfig(ctx context.Context) (*config.OTLPExporter, EnvVars, error) {
	envVars, err := makeEnvVars(ctx, cb.reader, cb.otlpOutput, cb.pipelineName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %v", err)
	}

	exportersConfig := makeExportersConfig(cb.otlpOutput, cb.pipelineName, envVars, cb.queueSize)
	return exportersConfig, envVars, nil
}

func makeExportersConfig(otlpOutput *telemetryv1alpha1.OtlpOutput, pipelineName string, envVars map[string][]byte, queueSize int) *config.OTLPExporter {
	headers := makeHeaders(otlpOutput, pipelineName)
	otlpEndpointVariable := makeOtlpEndpointVariable(pipelineName)
	otlpEndpointValue := string(envVars[otlpEndpointVariable])
	tlsConfig := makeTLSConfig(otlpOutput, otlpEndpointValue, pipelineName)

	otlpExporterConfig := config.OTLPExporter{
		Endpoint: fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:  headers,
		TLS:      tlsConfig,
		SendingQueue: config.SendingQueue{
			Enabled:   true,
			QueueSize: queueSize,
		},
		RetryOnFailure: config.RetryOnFailure{
			Enabled:         true,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			MaxElapsedTime:  "300s",
		},
	}

	return &otlpExporterConfig
}

func ExporterID(output *telemetryv1alpha1.OtlpOutput, pipelineName string) string {
	var outputType string
	if output.Protocol == "http" {
		outputType = "otlphttp"
	} else {
		outputType = "otlp"
	}

	return fmt.Sprintf("%s/%s", outputType, pipelineName)
}

func makeTLSConfig(output *telemetryv1alpha1.OtlpOutput, otlpEndpointValue, pipelineName string) config.TLS {
	var cfg config.TLS

	// TODO if endpoint is secure, but insecure is set to true, then the insecure setting should be ignored.
	// https://opentelemetry.io/docs/collector/configuration/
	// TODO do we even need to check if the endpoint is secure?
	if isInsecureEndpoint(otlpEndpointValue) || output.TLS.Insecure {
		cfg.Insecure = true
		return cfg
	}
	cfg.InsecureSkipVerify = output.TLS.InsecureSkipVerify
	if !cfg.InsecureSkipVerify && output.TLS.CA.IsDefined() {
		cfg.CAPem = fmt.Sprintf("${%s}", makeTLSConfigEnvVarCompliant(tlsConfigCaVariablePrefix, pipelineName))
	}
	if output.TLS.Cert.IsDefined() {
		cfg.CertPem = fmt.Sprintf("${%s}", makeTLSConfigEnvVarCompliant(tlsConfigCertVariablePrefix, pipelineName))
	}
	if output.TLS.Key.IsDefined() {
		cfg.KeyPem = fmt.Sprintf("${%s}", makeTLSConfigEnvVarCompliant(tlsConfigKeyVariablePrefix, pipelineName))
	}

	return cfg
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

func isInsecureEndpoint(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}
