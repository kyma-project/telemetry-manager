package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/collector"

	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FieldDescriptor struct {
	TargetSecretKey string
	SecretKeyRef    telemetryv1alpha1.SecretKeyRef
}

func FetchSecretData(ctx context.Context, c client.Reader, output *telemetryv1alpha1.OtlpOutput) (map[string][]byte, error) {
	secretData := map[string][]byte{}

	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		username, err := fetchSecretValue(ctx, c, output.Authentication.Basic.User)
		if err != nil {
			return nil, err
		}
		password, err := fetchSecretValue(ctx, c, output.Authentication.Basic.Password)
		if err != nil {
			return nil, err
		}
		basicAuthHeader := getBasicAuthHeader(string(username), string(password))
		secretData[collector.BasicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	endpoint, err := fetchSecretValue(ctx, c, output.Endpoint)
	if err != nil {
		return nil, err
	}
	secretData[collector.EndpointVariable] = endpoint

	for _, header := range output.Headers {
		key := fmt.Sprintf("HEADER_%s", envvar.MakeEnvVarCompliant(header.Name))
		value, err := fetchSecretValue(ctx, c, header.ValueType)
		if err != nil {
			return nil, err
		}
		secretData[key] = value
	}

	return secretData, nil
}

func fetchSecretValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}
	if value.ValueFrom.IsSecretKeyRef() {
		lookupKey := types.NamespacedName{
			Name:      value.ValueFrom.SecretKeyRef.Name,
			Namespace: value.ValueFrom.SecretKeyRef.Namespace,
		}

		var secret corev1.Secret
		if err := c.Get(ctx, lookupKey, &secret); err != nil {
			return nil, err
		}

		if secretValue, found := secret.Data[value.ValueFrom.SecretKeyRef.Key]; found {
			return secretValue, nil
		}
		return nil, fmt.Errorf("referenced key not found in Secret")
	}

	return nil, fmt.Errorf("either value or secretReference have to be defined")
}

func getBasicAuthHeader(username string, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

func LookupSecretRefFields(otlpOut *telemetryv1alpha1.OtlpOutput, name string) []FieldDescriptor {
	var result []FieldDescriptor

	if otlpOut.Endpoint.ValueFrom != nil && otlpOut.Endpoint.ValueFrom.IsSecretKeyRef() {

		result = append(result, FieldDescriptor{
			TargetSecretKey: otlpOut.Endpoint.ValueFrom.SecretKeyRef.Name,
			SecretKeyRef:    *otlpOut.Endpoint.ValueFrom.SecretKeyRef,
		})
	}

	if otlpOut.Authentication != nil && otlpOut.Authentication.Basic.IsDefined() {
		result = appendOutputFieldIfHasSecretRef(result, name, otlpOut.Authentication.Basic.User)
		result = appendOutputFieldIfHasSecretRef(result, name, otlpOut.Authentication.Basic.Password)
	}

	for _, header := range otlpOut.Headers {
		result = appendOutputFieldIfHasSecretRef(result, name, header.ValueType)
	}

	return result
}

func appendOutputFieldIfHasSecretRef(fields []FieldDescriptor, pipelineName string, valueType telemetryv1alpha1.ValueType) []FieldDescriptor {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		fields = append(fields, FieldDescriptor{
			TargetSecretKey: envvar.GenerateName(pipelineName, *valueType.ValueFrom.SecretKeyRef),
			SecretKeyRef:    *valueType.ValueFrom.SecretKeyRef,
		})
	}

	return fields
}

func ContainsAnyRefToSecret(pipeline *telemetryv1alpha1.TracePipeline, secret *corev1.Secret) bool {
	secretName := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}
	if pipeline.Spec.Output.Otlp.Endpoint.IsDefined() &&
		referencesSecret(pipeline.Spec.Output.Otlp.Endpoint, secretName) {
		return true
	}

	if pipeline.Spec.Output.Otlp == nil ||
		pipeline.Spec.Output.Otlp.Authentication == nil ||
		pipeline.Spec.Output.Otlp.Authentication.Basic == nil ||
		!pipeline.Spec.Output.Otlp.Authentication.Basic.IsDefined() {
		return false
	}

	auth := pipeline.Spec.Output.Otlp.Authentication.Basic

	return referencesSecret(auth.User, secretName) || referencesSecret(auth.Password, secretName)
}

func referencesSecret(valueType telemetryv1alpha1.ValueType, secretName types.NamespacedName) bool {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		return valueType.ValueFrom.SecretKeyRef.NamespacedName() == secretName
	}

	return false
}
