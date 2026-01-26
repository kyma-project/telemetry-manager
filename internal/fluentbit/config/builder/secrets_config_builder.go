package builder

import (
	"bytes"
	"context"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

// Copies HTTP-specific attributes and user-provided variables to a secret that is later used for providing environment variables to the Fluent Bit configuration.
func (b *ConfigBuilder) buildEnvConfigSecret(ctx context.Context, logPipelines []telemetryv1beta1.LogPipeline) (map[string][]byte, error) {
	envConfigSecret := make(map[string][]byte)

	for i := range logPipelines {
		if err := b.extractHTTPSecrets(ctx, &logPipelines[i], envConfigSecret); err != nil {
			return nil, err
		}

		// Extract variable secrets
		if err := b.extractVariableSecrets(ctx, &logPipelines[i], envConfigSecret); err != nil {
			return nil, err
		}
	}

	return envConfigSecret, nil
}

// Copies TLS-specific attributes to a secret, that is later mounted as a file, and used in the Fluent Bit configuration
// (since PEM-encoded strings exceed the maximum allowed length of environment variables on some Linux machines).
func (b *ConfigBuilder) buildTLSFileConfigSecret(ctx context.Context, logPipelines []telemetryv1beta1.LogPipeline) (map[string][]byte, error) {
	tlsSecretConfig := make(map[string][]byte)

	for i := range logPipelines {
		output := logPipelines[i].Spec.Output
		if !logpipelineutils.IsHTTPOutputDefined(&output) {
			continue
		}

		tlsConfig := output.FluentBitHTTP.TLS
		if sharedtypesutils.IsValid(tlsConfig.CA) {
			targetKey := fmt.Sprintf("%s-ca.crt", logPipelines[i].Name)

			caSecretData, err := getFromValueOrSecret(ctx, b.reader, *tlsConfig.CA, targetKey)
			if err != nil {
				return nil, fmt.Errorf("failed to get CA secret data: %w", err)
			}

			maps.Copy(tlsSecretConfig, caSecretData)
		}

		if sharedtypesutils.IsValid(tlsConfig.Cert) && sharedtypesutils.IsValid(tlsConfig.Key) {
			targetCertVariable := fmt.Sprintf("%s-cert.crt", logPipelines[i].Name)

			certSecretData, err := getFromValueOrSecret(ctx, b.reader, *tlsConfig.Cert, targetCertVariable)
			if err != nil {
				return nil, fmt.Errorf("failed to get cert secret data: %w", err)
			}

			targetKeyVariable := fmt.Sprintf("%s-key.key", logPipelines[i].Name)

			keySecretData, err := getFromValueOrSecret(ctx, b.reader, *tlsConfig.Key, targetKeyVariable)
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

// extractHTTPSecrets handles the extraction of secrets from HTTP output
func (b *ConfigBuilder) extractHTTPSecrets(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline, envSecretConfig map[string][]byte) error {
	httpOutput := pipeline.Spec.Output.FluentBitHTTP
	if httpOutput == nil {
		return nil
	}

	// Extract host secret if needed
	if shouldCopySecret(&httpOutput.Host) {
		hostSecret, err := getEnvConfigSecret(ctx, b.reader, pipeline.Name, &httpOutput.Host)
		if err != nil {
			return fmt.Errorf("failed to get host secret: %w", err)
		}

		maps.Copy(envSecretConfig, hostSecret)
	}

	// Extract user secret if needed
	if shouldCopySecret(httpOutput.User) {
		userSecret, err := getEnvConfigSecret(ctx, b.reader, pipeline.Name, httpOutput.User)
		if err != nil {
			return fmt.Errorf("failed to get user secret: %w", err)
		}

		maps.Copy(envSecretConfig, userSecret)
	}

	// Extract password secret if needed
	if shouldCopySecret(httpOutput.Password) {
		passwordSecret, err := getEnvConfigSecret(ctx, b.reader, pipeline.Name, httpOutput.Password)
		if err != nil {
			return fmt.Errorf("failed to get password secret: %w", err)
		}

		maps.Copy(envSecretConfig, passwordSecret)
	}

	return nil
}

// extractVariableSecrets handles the extraction of secrets from variables
func (b *ConfigBuilder) extractVariableSecrets(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline, envConfigSecret map[string][]byte) error {
	for _, ref := range pipeline.Spec.FluentBitVariables {
		if ref.ValueFrom.SecretKeyRef == nil {
			continue
		}

		variableSecret, err := getSecretData(ctx, b.reader, *ref.ValueFrom.SecretKeyRef, ref.Name)
		if err != nil {
			return fmt.Errorf("failed to get variable secret: %w", err)
		}

		maps.Copy(envConfigSecret, variableSecret)
	}

	return nil
}

func getEnvConfigSecret(ctx context.Context, client client.Reader, prefix string, value *telemetryv1beta1.ValueType) (map[string][]byte, error) {
	var ref = value.ValueFrom.SecretKeyRef

	targetKey := formatEnvVarName(prefix, ref.Namespace, ref.Name, ref.Key)

	secretData, err := getSecretData(ctx, client, *ref, targetKey)
	if err != nil {
		return nil, err
	}

	return secretData, nil
}

func getFromValueOrSecret(ctx context.Context, client client.Reader, value telemetryv1beta1.ValueType, targetKey string) (map[string][]byte, error) {
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

func getSecretData(ctx context.Context, client client.Reader, ref telemetryv1beta1.SecretKeyRef, targetKey string) (map[string][]byte, error) {
	var source corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, &source); err != nil {
		return nil, fmt.Errorf("unable to read secret '%s' from namespace '%s': %w", ref.Name, ref.Namespace, err)
	}

	if val, found := source.Data[ref.Key]; found {
		return map[string][]byte{
			targetKey: val,
		}, nil
	}

	return nil, fmt.Errorf("unable to find key '%s' in secret '%s' from namespace '%s'",
		ref.Key,
		ref.Name,
		ref.Namespace)
}

func shouldCopySecret(value *telemetryv1beta1.ValueType) bool {
	return value != nil &&
		value.Value == "" &&
		value.ValueFrom != nil &&
		value.ValueFrom.SecretKeyRef != nil
}
