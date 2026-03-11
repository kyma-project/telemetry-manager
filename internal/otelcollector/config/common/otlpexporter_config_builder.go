package common

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

// =============================================================================
// OTLP EXPORTER CONFIG BUILDER
// =============================================================================

type OTLPExporterConfigBuilder struct {
	reader       client.Reader
	otlpOutput   *telemetryv1beta1.OTLPOutput
	pipelineName string
	queueSize    int
	signalType   SignalType
}

type EnvVars map[string][]byte

func NewOTLPExporterConfigBuilder(reader client.Reader, otlpOutput *telemetryv1beta1.OTLPOutput, pipelineName string, queueSize int, signalType SignalType) *OTLPExporterConfigBuilder {
	return &OTLPExporterConfigBuilder{
		reader:       reader,
		otlpOutput:   otlpOutput,
		pipelineName: pipelineName,
		queueSize:    queueSize,
		signalType:   signalType,
	}
}

func (cb *OTLPExporterConfigBuilder) OTLPExporter(ctx context.Context) (*OTLPExporterConfig, EnvVars, error) {
	envVars, err := makeOTLPExporterEnvVars(ctx, cb.reader, cb.otlpOutput, cb.pipelineName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %w", err)
	}

	exporter := otlpExporter(cb.otlpOutput, cb.pipelineName, envVars, cb.queueSize, cb.signalType)

	return exporter, envVars, nil
}

func otlpExporter(otlpOutput *telemetryv1beta1.OTLPOutput, pipelineName string, envVars map[string][]byte, queueSize int, signalType SignalType) *OTLPExporterConfig {
	headers := headers(otlpOutput, pipelineName)
	otlpEndpointVariable := formatEnvVarKey(otlpEndpointVariablePrefix, pipelineName)
	otlpEndpointValue := string(envVars[otlpEndpointVariable])
	tls := tls(otlpOutput, otlpEndpointValue, pipelineName)

	sendingQueue := SendingQueue{
		Enabled: false,
	}
	if queueSize != 0 {
		sendingQueue.QueueSize = queueSize
		sendingQueue.Enabled = true
	}

	otlpExporter := OTLPExporterConfig{
		Endpoint:     fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:      headers,
		TLS:          tls,
		SendingQueue: sendingQueue,
		RetryOnFailure: RetryOnFailure{
			Enabled:         true,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			MaxElapsedTime:  "300s",
		},
	}

	if len(otlpOutput.Path) > 0 && SignalTypeMetric == signalType {
		otlpExporter.Endpoint = ""
		otlpExporter.MetricsEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if len(otlpOutput.Path) > 0 && SignalTypeTrace == signalType {
		otlpExporter.Endpoint = ""
		otlpExporter.TracesEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if len(otlpOutput.Path) > 0 && SignalTypeLog == signalType {
		otlpExporter.Endpoint = ""
		otlpExporter.LogsEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if otlpOutput.Authentication != nil && otlpOutput.Authentication.OAuth2 != nil {
		otlpExporter.Auth = Auth{
			Authenticator: fmt.Sprintf(ComponentIDOAuth2Extension, pipelineName),
		}
	}

	return &otlpExporter
}

func ExporterID(protocol telemetryv1beta1.OTLPProtocol, pipelineName string) string {
	if protocol == telemetryv1beta1.OTLPProtocolHTTP {
		return fmt.Sprintf(ComponentIDOTLPHTTPExporter, pipelineName)
	}

	return fmt.Sprintf(ComponentIDOTLPGRPCExporter, pipelineName)
}

func tls(output *telemetryv1beta1.OTLPOutput, otlpEndpointValue, pipelineName string) TLS {
	var config TLS

	config.Insecure = isInsecureOutput(otlpEndpointValue)

	if output.TLS == nil {
		return config
	}

	if !config.Insecure {
		config.Insecure = output.TLS.Insecure
	}

	config.InsecureSkipVerify = output.TLS.InsecureSkipVerify
	if sharedtypesutils.IsValid(output.TLS.CA) {
		config.CAPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCaVariablePrefix, pipelineName))
	}

	if sharedtypesutils.IsValid(output.TLS.Cert) {
		config.CertPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCertVariablePrefix, pipelineName))
	}

	if sharedtypesutils.IsValid(output.TLS.Key) {
		config.KeyPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigKeyVariablePrefix, pipelineName))
	}

	return config
}

func headers(output *telemetryv1beta1.OTLPOutput, pipelineName string) map[string]string {
	headers := make(map[string]string)

	if isBasicAuthEnabled(output.Authentication) {
		basicAuthHeaderVariable := formatEnvVarKey(basicAuthHeaderVariablePrefix, pipelineName)
		headers["Authorization"] = fmt.Sprintf("${%s}", basicAuthHeaderVariable)
	}

	for _, header := range output.Headers {
		headers[header.Name] = fmt.Sprintf("${%s}", formatHeaderEnvVarKey(header, pipelineName))
	}

	return headers
}

func isInsecureOutput(endpoint string) bool {
	return len(strings.TrimSpace(endpoint)) > 0 && strings.HasPrefix(endpoint, "http://")
}

func isBasicAuthEnabled(authOptions *telemetryv1beta1.AuthenticationOptions) bool {
	return authOptions != nil &&
		authOptions.Basic != nil &&
		sharedtypesutils.IsValid(&authOptions.Basic.User) &&
		sharedtypesutils.IsValid(&authOptions.Basic.Password)
}
