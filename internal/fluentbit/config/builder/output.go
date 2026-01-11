package builder

import (
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

// Considering Fluent Bit's exponential back-off and jitter algorithm with the default scheduler.base and scheduler.cap,
// this retry limit should be enough to cover about 3 days of retrying. See
// https://docs.fluentbit.io/manual/administration/scheduling-and-retries. We do not want unlimited retries to avoid
// that malformed logs stay in the buffer forever.
var retryLimit = "300"

func createOutputSection(pipeline *telemetryv1beta1.LogPipeline, defaults pipelineDefaults) string {
	output := &pipeline.Spec.Output
	if logpipelineutils.IsCustomOutputDefined(output) {
		return generateCustomOutput(output, defaults.FsBufferLimit, pipeline.Name)
	}

	if logpipelineutils.IsHTTPOutputDefined(output) {
		return generateHTTPOutput(output.FluentBitHTTP, defaults.FsBufferLimit, pipeline.Name)
	}

	return ""
}

func generateCustomOutput(output *telemetryv1beta1.LogPipelineOutput, fsBufferLimit string, name string) string {
	sb := NewOutputSectionBuilder()
	customOutputParams := parseMultiline(output.FluentBitCustom)
	aliasPresent := customOutputParams.ContainsKey("alias")

	for _, p := range customOutputParams {
		sb.AddConfigParam(p.Key, p.Value)
	}

	if !aliasPresent {
		sb.AddConfigParam("alias", name)
	}

	sb.AddConfigParam("match", fmt.Sprintf("%s.*", name))
	sb.AddConfigParam("storage.total_limit_size", fsBufferLimit)
	sb.AddConfigParam("retry_limit", retryLimit)

	return sb.Build()
}

func generateHTTPOutput(httpOutput *telemetryv1beta1.FluentBitHTTPOutput, fsBufferLimit string, name string) string {
	sb := NewOutputSectionBuilder()
	sb.AddConfigParam("name", "http")
	sb.AddConfigParam("allow_duplicated_headers", "true")
	sb.AddConfigParam("match", fmt.Sprintf("%s.*", name))
	sb.AddConfigParam("alias", name)
	sb.AddConfigParam("storage.total_limit_size", fsBufferLimit)
	sb.AddConfigParam("retry_limit", retryLimit)
	sb.AddIfNotEmpty("uri", httpOutput.URI)
	sb.AddIfNotEmpty("compress", httpOutput.Compress)
	sb.AddIfNotEmptyOrDefault("port", httpOutput.Port, "443")
	sb.AddIfNotEmptyOrDefault("format", httpOutput.Format, "json")
	sb.AddConfigParam("json_date_format", "iso8601")

	if sharedtypesutils.IsValid(&httpOutput.Host) {
		value := resolveValue(httpOutput.Host, name)
		sb.AddConfigParam("host", value)
	}

	if sharedtypesutils.IsValid(httpOutput.Password) {
		value := resolveValue(*httpOutput.Password, name)
		sb.AddConfigParam("http_passwd", value)
	}

	if sharedtypesutils.IsValid(httpOutput.User) {
		value := resolveValue(*httpOutput.User, name)
		sb.AddConfigParam("http_user", value)
	}

	tlsEnabled := "on"
	if httpOutput.TLS.Insecure {
		tlsEnabled = "off"
	}

	sb.AddConfigParam("tls", tlsEnabled)

	tlsVerify := "on"
	if httpOutput.TLS.InsecureSkipVerify {
		tlsVerify = "off"
	}

	sb.AddConfigParam("tls.verify", tlsVerify)

	if sharedtypesutils.IsValid(httpOutput.TLS.CA) {
		sb.AddConfigParam("tls.ca_file", fmt.Sprintf("/fluent-bit/etc/output-tls-config/%s-ca.crt", name))
	}

	if sharedtypesutils.IsValid(httpOutput.TLS.Cert) {
		sb.AddConfigParam("tls.crt_file", fmt.Sprintf("/fluent-bit/etc/output-tls-config/%s-cert.crt", name))
	}

	if sharedtypesutils.IsValid(httpOutput.TLS.Key) {
		sb.AddConfigParam("tls.key_file", fmt.Sprintf("/fluent-bit/etc/output-tls-config/%s-key.key", name))
	}

	return sb.Build()
}

func resolveValue(value telemetryv1beta1.ValueType, logPipeline string) string {
	if value.Value != "" {
		return value.Value
	}

	if value.ValueFrom != nil && sharedtypesutils.IsValid(&value) {
		secretKeyRef := value.ValueFrom.SecretKeyRef
		return fmt.Sprintf("${%s}", formatEnvVarName(logPipeline, secretKeyRef.Namespace, secretKeyRef.Name, secretKeyRef.Key))
	}

	return ""
}
