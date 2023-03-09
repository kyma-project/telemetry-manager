package collector

import (
	"context"
	"encoding/base64"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type FieldDescriptor struct {
	TargetSecretKey string
	SecretKeyRef    telemetryv1alpha1.SecretKeyRef
}

func FetchSecretData(ctx context.Context, client client.Reader, output *telemetryv1alpha1.OtlpOutput) (map[string][]byte, error) {
	secretData := map[string][]byte{}

	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		username, err := fetchSecretValue(ctx, client, output.Authentication.Basic.User)
		if err != nil {
			return nil, err
		}
		password, err := fetchSecretValue(ctx, client, output.Authentication.Basic.Password)
		if err != nil {
			return nil, err
		}
		basicAuthHeader := getBasicAuthHeader(string(username), string(password))
		secretData[BasicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	endpoint, err := fetchSecretValue(ctx, client, output.Endpoint)
	if err != nil {
		return nil, err
	}
	secretData[EndpointVariable] = endpoint

	for _, header := range output.Headers {
		key := fmt.Sprintf("HEADER_%s", envvar.MakeEnvVarCompliant(header.Name))
		value, err := fetchSecretValue(ctx, client, header.ValueType)
		if err != nil {
			return nil, err
		}
		secretData[key] = value
	}

	return secretData, nil
}

func fetchSecretValue(ctx context.Context, client client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}
	if value.ValueFrom.IsSecretKeyRef() {
		lookupKey := types.NamespacedName{
			Name:      value.ValueFrom.SecretKeyRef.Name,
			Namespace: value.ValueFrom.SecretKeyRef.Namespace,
		}

		var secret corev1.Secret
		if err := client.Get(ctx, lookupKey, &secret); err != nil {
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

func CheckForMissingSecrets(ctx context.Context, client client.Client, pipelineName string, otlpOutput *telemetryv1alpha1.OtlpOutput) bool {
	secretRefFields := lookupSecretRefFields(otlpOutput, pipelineName)
	for _, field := range secretRefFields {
		hasKey := checkSecretHasKey(ctx, client, field.SecretKeyRef)
		if !hasKey {
			return true
		}
	}

	return false
}

func lookupSecretRefFields(otlpOut *telemetryv1alpha1.OtlpOutput, name string) []FieldDescriptor {
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

func checkSecretHasKey(ctx context.Context, client client.Client, from telemetryv1alpha1.SecretKeyRef) bool {
	log := logf.FromContext(ctx)

	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: from.Name, Namespace: from.Namespace}, &secret); err != nil {
		log.V(1).Info(fmt.Sprintf("Unable to get secret '%s' from namespace '%s'", from.Name, from.Namespace))
		return false
	}
	if _, ok := secret.Data[from.Key]; !ok {
		log.V(1).Info(fmt.Sprintf("Unable to find key '%s' in secret '%s'", from.Key, from.Name))
		return false
	}

	return true
}

func HasSecretRef(otlpOut *telemetryv1alpha1.OtlpOutput, secretName types.NamespacedName) bool {
	if otlpOut.Endpoint.IsDefined() && referencesSecret(otlpOut.Endpoint, secretName) {
		return true
	}

	if otlpOut == nil ||
		otlpOut.Authentication == nil ||
		otlpOut.Authentication.Basic == nil ||
		!otlpOut.Authentication.Basic.IsDefined() {
		return false
	}

	basicAuth := otlpOut.Authentication.Basic
	return referencesSecret(basicAuth.User, secretName) || referencesSecret(basicAuth.Password, secretName)

}

func referencesSecret(valueType telemetryv1alpha1.ValueType, secretName types.NamespacedName) bool {
	if valueType.Value == "" && valueType.ValueFrom != nil && valueType.ValueFrom.IsSecretKeyRef() {
		return valueType.ValueFrom.SecretKeyRef.NamespacedName() == secretName
	}

	return false
}
