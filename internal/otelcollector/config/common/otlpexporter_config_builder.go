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
	reader      client.Reader
	otlpOutput  *telemetryv1beta1.OTLPOutput
	pipelineRef PipelineRef
	queueSize   int
}

type EnvVars map[string][]byte

func NewOTLPExporterConfigBuilder(reader client.Reader, otlpOutput *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef, queueSize int) *OTLPExporterConfigBuilder {
	return &OTLPExporterConfigBuilder{
		reader:      reader,
		otlpOutput:  otlpOutput,
		pipelineRef: pipelineRef,
		queueSize:   queueSize,
	}
}

func (cb *OTLPExporterConfigBuilder) OTLPExporter(ctx context.Context) (*OTLPExporterConfig, EnvVars, error) {
	envVars, err := makeOTLPExporterEnvVars(ctx, cb.reader, cb.otlpOutput, cb.pipelineRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %w", err)
	}

	exporter := otlpExporter(cb.otlpOutput, cb.pipelineRef, envVars, cb.queueSize)

	return exporter, envVars, nil
}

func otlpExporter(otlpOutput *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef, envVars map[string][]byte, queueSize int) *OTLPExporterConfig {
	otlpEndpointVariable := formatEnvVarKey(otlpEndpointVariablePrefix, pipelineRef)
	otlpEndpointValue := string(envVars[otlpEndpointVariable])

	sendingQueue := SendingQueue{
		Enabled: false,
	}
	if queueSize != 0 {
		sendingQueue.QueueSize = queueSize
		sendingQueue.Enabled = true
	}

	compression := string(otlpOutput.Compression)
	if compression == "" {
		compression = string(telemetryv1beta1.OTLPCompressionGzip)
	}

	exporter := OTLPExporterConfig{
		Endpoint:     fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:      headers(otlpOutput, pipelineRef),
		TLS:          tls(otlpOutput, otlpEndpointValue, pipelineRef),
		Compression:  compression,
		SendingQueue: sendingQueue,
		RetryOnFailure: RetryOnFailure{
			Enabled:         true,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			MaxElapsedTime:  "300s",
		},
	}

	if len(otlpOutput.Path) > 0 && SignalTypeMetric == pipelineRef.signalType {
		exporter.Endpoint = ""
		exporter.MetricsEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if len(otlpOutput.Path) > 0 && SignalTypeTrace == pipelineRef.signalType {
		exporter.Endpoint = ""
		exporter.TracesEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if len(otlpOutput.Path) > 0 && SignalTypeLog == pipelineRef.signalType {
		exporter.Endpoint = ""
		exporter.LogsEndpoint = fmt.Sprintf("${%s}", otlpEndpointVariable)
	}

	if otlpOutput.Authentication != nil && otlpOutput.Authentication.OAuth2 != nil {
		exporter.Auth = Auth{
			Authenticator: ComponentIDOAuth2Extension(pipelineRef.qualifiedName()),
		}
	}

	return &exporter
}

func OTLPExporterID(protocol telemetryv1beta1.OTLPProtocol, pipelineRef PipelineRef) string {
	if protocol == telemetryv1beta1.OTLPProtocolHTTP {
		return ComponentIDOTLPHTTPExporter(pipelineRef.qualifiedName())
	}

	return ComponentIDOTLPGRPCExporter(pipelineRef.qualifiedName())
}

func tls(output *telemetryv1beta1.OTLPOutput, otlpEndpointValue string, pipelineRef PipelineRef) TLS {
	var tls TLS

	tls.Insecure = isInsecureOutput(otlpEndpointValue)

	if output.TLS == nil {
		return tls
	}

	if !tls.Insecure {
		tls.Insecure = output.TLS.Insecure
	}

	tls.InsecureSkipVerify = output.TLS.InsecureSkipVerify
	if sharedtypesutils.IsValid(output.TLS.CA) {
		tls.CAPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCaVariablePrefix, pipelineRef))
	}

	if sharedtypesutils.IsValid(output.TLS.Cert) {
		tls.CertPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCertVariablePrefix, pipelineRef))
	}

	if sharedtypesutils.IsValid(output.TLS.Key) {
		tls.KeyPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigKeyVariablePrefix, pipelineRef))
	}

	return tls
}

func headers(output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) map[string]string {
	headers := make(map[string]string)

	if isBasicAuthEnabled(output.Authentication) {
		basicAuthHeaderVariable := formatEnvVarKey(basicAuthHeaderVariablePrefix, pipelineRef)
		headers["Authorization"] = fmt.Sprintf("${%s}", basicAuthHeaderVariable)
	}

	for _, header := range output.Headers {
		headers[header.Name] = fmt.Sprintf("${%s}", formatHeaderEnvVarKey(header, pipelineRef))
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
