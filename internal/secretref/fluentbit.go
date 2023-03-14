package secretref

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func LogPipelineReferencesNonExistentSecret(ctx context.Context, client client.Reader, pipeline *telemetryv1alpha1.LogPipeline) bool {
	secretRefFields := GetRefsInLogPipeline(pipeline)
	for _, field := range secretRefFields {
		hasKey := checkIfSecretHasKey(ctx, client, field.SecretKeyRef)
		if !hasKey {
			return true
		}
	}

	return false
}

func LogPipelineReferencesSecret(secretName, secretNamespace string, pipeline *telemetryv1alpha1.LogPipeline) bool {
	fields := GetRefsInLogPipeline(pipeline)

	for _, field := range fields {
		if field.SecretKeyRef.Name == secretName && field.SecretKeyRef.Namespace == secretNamespace {
			return true
		}
	}

	return false
}

func GetRefsInLogPipeline(pipeline *telemetryv1alpha1.LogPipeline) []FieldDescriptor {
	var fields []FieldDescriptor

	for _, v := range pipeline.Spec.Variables {
		if !v.ValueFrom.IsSecretKeyRef() {
			continue
		}

		fields = append(fields, FieldDescriptor{
			TargetSecretKey: v.Name,
			SecretKeyRef:    *v.ValueFrom.SecretKeyRef,
		})
	}

	output := pipeline.Spec.Output
	if output.IsHTTPDefined() {
		fields = appendIfSecretRef(fields, pipeline.Name, output.HTTP.Host)
		fields = appendIfSecretRef(fields, pipeline.Name, output.HTTP.User)
		fields = appendIfSecretRef(fields, pipeline.Name, output.HTTP.Password)
	}
	if output.IsLokiDefined() {
		fields = appendIfSecretRef(fields, pipeline.Name, output.Loki.URL)
	}

	return fields
}
