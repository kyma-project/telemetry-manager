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
	namePrefix   string
}

type EnvVars map[string][]byte

func NewOTLPExporterConfigBuilder(reader client.Reader, otlpOutput *telemetryv1beta1.OTLPOutput, pipelineName string, queueSize int, signalType SignalType, namePrefix string) *OTLPExporterConfigBuilder {
	return &OTLPExporterConfigBuilder{
		reader:       reader,
		otlpOutput:   otlpOutput,
		pipelineName: pipelineName,
		queueSize:    queueSize,
		signalType:   signalType,
		namePrefix:   namePrefix,
	}
}

func (cb *OTLPExporterConfigBuilder) OTLPExporter(ctx context.Context) (*OTLPExporterConfig, EnvVars, error) {
	envVars, err := makeOTLPExporterEnvVars(ctx, cb.reader, cb.otlpOutput, cb.pipelineName, cb.signalType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %w", err)
	}

	exporter := otlpExporter(cb.otlpOutput, cb.pipelineName, envVars, cb.queueSize, cb.signalType, cb.namePrefix)

	return exporter, envVars, nil
}

func otlpExporter(otlpOutput *telemetryv1beta1.OTLPOutput, pipelineName string, envVars map[string][]byte, queueSize int, signalType SignalType, namePrefix string) *OTLPExporterConfig {
	qualifiedName := prefixedName(namePrefix, pipelineName)
	headers := headers(otlpOutput, signalType, pipelineName)
	otlpEndpointVariable := formatEnvVarKey(otlpEndpointVariablePrefix, signalType, pipelineName)
	otlpEndpointValue := string(envVars[otlpEndpointVariable])
	tls := tls(otlpOutput, otlpEndpointValue, signalType, pipelineName)

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

	otlpExporter := OTLPExporterConfig{
		Endpoint:     fmt.Sprintf("${%s}", otlpEndpointVariable),
		Headers:      headers,
		TLS:          tls,
		Compression:  compression,
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
			Authenticator: fmt.Sprintf(ComponentIDOAuth2Extension, qualifiedName),
		}
	}

	return &otlpExporter
}

func ExporterID(protocol telemetryv1beta1.OTLPProtocol, pipelineName, namePrefix string) string {
	qualifiedName := prefixedName(namePrefix, pipelineName)
	if protocol == telemetryv1beta1.OTLPProtocolHTTP {
		return fmt.Sprintf(ComponentIDOTLPHTTPExporter, qualifiedName)
	}

	return fmt.Sprintf(ComponentIDOTLPGRPCExporter, qualifiedName)
}

// prefixedName returns "prefix-name" when prefix is non-empty, or just "name" when prefix is empty.
func prefixedName(namePrefix, pipelineName string) string {
	if namePrefix == "" {
		return pipelineName
	}

	return namePrefix + "-" + pipelineName
}

func UserDefinedTransformProcessorID(pipelineName, namePrefix string) string {
	return fmt.Sprintf(ComponentIDUserDefinedTransformProcessor, prefixedName(namePrefix, "user-defined-"+pipelineName))
}

func UserDefinedFilterProcessorID(pipelineName, namePrefix string) string {
	return fmt.Sprintf(ComponentIDUserDefinedFilterProcessor, prefixedName(namePrefix, "user-defined-"+pipelineName))
}

func tls(output *telemetryv1beta1.OTLPOutput, otlpEndpointValue string, signalType SignalType, pipelineName string) TLS {
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
		tls.CAPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCaVariablePrefix, signalType, pipelineName))
	}

	if sharedtypesutils.IsValid(output.TLS.Cert) {
		tls.CertPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigCertVariablePrefix, signalType, pipelineName))
	}

	if sharedtypesutils.IsValid(output.TLS.Key) {
		tls.KeyPem = fmt.Sprintf("${%s}", formatEnvVarKey(tlsConfigKeyVariablePrefix, signalType, pipelineName))
	}

	return tls
}

func headers(output *telemetryv1beta1.OTLPOutput, signalType SignalType, pipelineName string) map[string]string {
	headers := make(map[string]string)

	if isBasicAuthEnabled(output.Authentication) {
		basicAuthHeaderVariable := formatEnvVarKey(basicAuthHeaderVariablePrefix, signalType, pipelineName)
		headers["Authorization"] = fmt.Sprintf("${%s}", basicAuthHeaderVariable)
	}

	for _, header := range output.Headers {
		headers[header.Name] = fmt.Sprintf("${%s}", formatHeaderEnvVarKey(header, signalType, pipelineName))
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
