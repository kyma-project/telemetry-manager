package metricpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ContainsAnyRefToSecret(pipeline *telemetryv1alpha1.MetricPipeline, secret *corev1.Secret) bool {
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
