package builder

import (
	"bytes"
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (b *AgentConfigBuilder) BuildEnvConfigSecret(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline) (map[string][]byte, error) {
	var envSecretConfig map[string][]byte
	for i := range logPipelines {
		if !logPipelines[i].DeletionTimestamp.IsZero() {
			continue
		}

		var httpOutput = logPipelines[i].Spec.Output.HTTP
		if httpOutput != nil {
			if shouldCopySecret(httpOutput.Host) {
				hostSecret, err := getEnvConfigSecret(ctx, b.Reader, logPipelines[i].Name, &httpOutput.Host)
				if err != nil {
					return nil, fmt.Errorf("failed to get host secret: %w", err)
				}
				maps.Copy(envSecretConfig, hostSecret)
			}
			if shouldCopySecret(httpOutput.User) {
				userSecret, err := getEnvConfigSecret(ctx, b.Reader, logPipelines[i].Name, &httpOutput.User)
				if err != nil {
					return nil, fmt.Errorf("failed to get user secret: %w", err)
				}
				maps.Copy(envSecretConfig, userSecret)
			}
			if shouldCopySecret(httpOutput.Password) {
				passwordSecret, err := getEnvConfigSecret(ctx, b.Reader, logPipelines[i].Name, &httpOutput.Password)
				if err != nil {
					return nil, fmt.Errorf("failed to get password secret: %w", err)
				}
				maps.Copy(envSecretConfig, passwordSecret)
			}
		}

		for _, ref := range logPipelines[i].Spec.Variables {
			if ref.ValueFrom.SecretKeyRef != nil {
				variableSecret, err := getSecretData(ctx, b.Reader, *ref.ValueFrom.SecretKeyRef, ref.Name)
				if err != nil {
					return nil, fmt.Errorf("failed to get variable secret: %w", err)
				}
				maps.Copy(envSecretConfig, variableSecret)
			}
		}
	}

	return envSecretConfig, nil
}

func (b *AgentConfigBuilder) BuildTLSFileConfigSecret(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline) (map[string][]byte, error) {
	var tlsSecretConfig map[string][]byte

	for i := range logPipelines {
		if !logPipelines[i].DeletionTimestamp.IsZero() {
			continue
		}

		output := logPipelines[i].Spec.Output
		if !logpipelineutils.IsHTTPDefined(&output) {
			continue
		}

		tlsConfig := output.HTTP.TLS
		if sharedtypesutils.IsValid(tlsConfig.CA) {
			targetKey := fmt.Sprintf("%s-ca.crt", logPipelines[i].Name)
			caSecretData, err := getFromValueOrSecret(ctx, b.Reader, *tlsConfig.CA, targetKey)
			if err != nil {
				return nil, fmt.Errorf("failed to get CA secret data: %w", err)
			}
			maps.Copy(tlsSecretConfig, caSecretData)
		}

		if sharedtypesutils.IsValid(tlsConfig.Cert) && sharedtypesutils.IsValid(tlsConfig.Key) {
			targetCertVariable := fmt.Sprintf("%s-cert.crt", logPipelines[i].Name)
			certSecretData, err := getFromValueOrSecret(ctx, b.Reader, *tlsConfig.Cert, targetCertVariable)
			if err != nil {
				return nil, fmt.Errorf("failed to get cert secret data: %w", err)
			}

			targetKeyVariable := fmt.Sprintf("%s-key.key", logPipelines[i].Name)
			keySecretData, err := getFromValueOrSecret(ctx, b.Reader, *tlsConfig.Key, targetKeyVariable)
			if err != nil {
				return nil, fmt.Errorf("failed to get key secret data: %w", err)
			}

			// Make a best effort replacement of linebreaks in cert/key if present.
			certSecretData[targetCertVariable] = bytes.ReplaceAll(certSecretData[targetCertVariable], []byte("\\n"), []byte("\n"))
			keySecretData[targetKeyVariable] = bytes.ReplaceAll(keySecretData[targetKeyVariable], []byte("\\n"), []byte("\n"))

			maps.Copy(tlsSecretConfig, certSecretData)
			maps.Copy(tlsSecretConfig, keySecretData)
		}
	}
	return tlsSecretConfig, nil
}

func getEnvConfigSecret(ctx context.Context, client client.Reader, prefix string, value *telemetryv1alpha1.ValueType) (map[string][]byte, error) {
	var ref = value.ValueFrom.SecretKeyRef

	targetKey := FormatEnvVarName(prefix, ref.Namespace, ref.Name, ref.Key)

	secretData, err := getSecretData(ctx, client, *ref, targetKey)
	if err != nil {
		return nil, err
	}
	return secretData, nil

}

func getFromValueOrSecret(ctx context.Context, client client.Reader, value telemetryv1alpha1.ValueType, targetKey string) (map[string][]byte, error) {
	if value.Value != "" {
		return map[string][]byte{
			targetKey: []byte(value.Value),
		}, nil
	}

	secretData, err := getSecretData(ctx, client, *value.ValueFrom.SecretKeyRef, targetKey)
	if err != nil {
		return nil, fmt.Errorf("unable to get secret data: %w", err)
	}

	return secretData, nil
}

func getSecretData(ctx context.Context, client client.Reader, ref telemetryv1alpha1.SecretKeyRef, targetKey string) (map[string][]byte, error) {
	var source corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &source); err != nil {
		return nil, fmt.Errorf("unable to read secret '%s' from namespace '%s': %w", ref.Name, ref.Namespace, err)
	}

	if val, found := source.Data[targetKey]; found {
		return map[string][]byte{
			targetKey: val,
		}, nil
	}

	return nil, fmt.Errorf("unable to find key '%s' in secret '%s' from namespace '%s'",
		ref.Key,
		ref.Name,
		ref.Namespace)
}

func shouldCopySecret(value telemetryv1alpha1.ValueType) bool {
	return value.Value != "" ||
		value.ValueFrom == nil ||
		value.ValueFrom.SecretKeyRef == nil
}
