package fluentbit

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

type syncer struct {
	client.Client
	config Config
}

func (s *syncer) syncFluentBitConfig(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, deployableLogPipelines []telemetryv1alpha1.LogPipeline) error {
	if len(deployableLogPipelines) == 0 {
		return nil
	}

	log := logf.FromContext(ctx)

	if err := s.syncSectionsConfigMap(ctx, pipeline, deployableLogPipelines); err != nil {
		return fmt.Errorf("failed to sync sections: %w", err)
	}

	if err := s.syncFilesConfigMap(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to sync mounted files: %w", err)
	}

	if err := s.syncEnvConfigSecret(ctx, deployableLogPipelines); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("referenced secret not found: %v", err))
			return nil
		}

		return err
	}

	if err := s.syncTLSFileConfigSecret(ctx, deployableLogPipelines); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("referenced tls config secret not found: %v", err))
			return nil
		}

		return err
	}

	return nil
}

func (s *syncer) syncSectionsConfigMap(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, deployablePipelines []telemetryv1alpha1.LogPipeline) error {
	cm, err := k8sutils.GetOrCreateConfigMap(ctx, s, s.config.SectionsConfigMap)
	if err != nil {
		return fmt.Errorf("unable to get section configmap: %w", err)
	}

	cmKey := pipeline.Name + ".conf"

	if !isLogPipelineDeployable(deployablePipelines, pipeline) || !pipeline.DeletionTimestamp.IsZero() {
		delete(cm.Data, cmKey)
	} else {
		builderConfig := builder.BuilderConfig{
			PipelineDefaults: s.config.PipelineDefaults,
			CollectAgentLogs: s.config.Overrides.Logging.CollectAgentLogs,
		}

		newConfig, err := builder.BuildFluentBitConfig(pipeline, builderConfig)
		if err != nil {
			return fmt.Errorf("unable to build section: %w", err)
		}

		if cm.Data == nil {
			cm.Data = map[string]string{cmKey: newConfig}
		} else if oldConfig, hasKey := cm.Data[cmKey]; !hasKey || oldConfig != newConfig {
			cm.Data[cmKey] = newConfig
		}
	}

	if err = controllerutil.SetOwnerReference(pipeline, &cm, s.Scheme()); err != nil {
		return fmt.Errorf("unable to set owner reference for section configmap: %w", err)
	}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update section configmap: %w", err)
	}

	return nil
}

func (s *syncer) syncFilesConfigMap(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	cm, err := k8sutils.GetOrCreateConfigMap(ctx, s, s.config.FilesConfigMap)
	if err != nil {
		return fmt.Errorf("unable to get files configmap: %w", err)
	}

	for _, file := range pipeline.Spec.Files {
		if pipeline.DeletionTimestamp != nil {
			delete(cm.Data, file.Name)
		} else {
			if cm.Data == nil {
				cm.Data = map[string]string{file.Name: file.Content}
			} else if oldContent, hasKey := cm.Data[file.Name]; !hasKey || oldContent != file.Content {
				cm.Data[file.Name] = file.Content
			}
		}
	}

	if pipeline.DeletionTimestamp.IsZero() {
		if err = controllerutil.SetOwnerReference(pipeline, &cm, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for files configmap: %w", err)
		}
	}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update files configmap: %w", err)
	}

	return nil
}

// Copies HTTP-specific attributes and user-provided variables to a secret that is later used for providing environment variables to the Fluent Bit configuration.
func (s *syncer) syncEnvConfigSecret(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline) error {
	oldSecret, err := k8sutils.GetOrCreateSecret(ctx, s, s.config.EnvConfigSecret)
	if err != nil {
		return fmt.Errorf("unable to get env secret: %w", err)
	}

	newSecret := oldSecret
	newSecret.Data = make(map[string][]byte)

	for i := range logPipelines {
		if !logPipelines[i].DeletionTimestamp.IsZero() {
			continue
		}

		var httpOutput = logPipelines[i].Spec.Output.HTTP
		if httpOutput != nil {
			if copyErr := s.copyConfigSecretData(ctx, logPipelines[i].Name, &httpOutput.Host, &newSecret); copyErr != nil {
				return copyErr
			}

			if copyErr := s.copyConfigSecretData(ctx, logPipelines[i].Name, &httpOutput.User, &newSecret); copyErr != nil {
				return copyErr
			}

			if copyErr := s.copyConfigSecretData(ctx, logPipelines[i].Name, &httpOutput.Password, &newSecret); copyErr != nil {
				return copyErr
			}
		}

		// we also store the variables in the env secret
		for _, ref := range logPipelines[i].Spec.Variables {
			if ref.ValueFrom.SecretKeyRef != nil {
				if copyErr := s.copySecretData(ctx, *ref.ValueFrom.SecretKeyRef, ref.Name, newSecret.Data); copyErr != nil {
					return fmt.Errorf("unable to copy secret data: %w", copyErr)
				}
			}
		}

		if err = controllerutil.SetOwnerReference(&logPipelines[i], &newSecret, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for env secret: %w", err)
		}
	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update env secret: %w", err)
	}

	return nil
}

func (s *syncer) copyConfigSecretData(ctx context.Context, prefix string, value *telemetryv1alpha1.ValueType, newSecret *corev1.Secret) error {
	if value.Value != "" || value.ValueFrom == nil || value.ValueFrom.SecretKeyRef == nil {
		return nil
	}

	var ref = value.ValueFrom.SecretKeyRef

	targetKey := builder.FormatEnvVarName(prefix, ref.Namespace, ref.Name, ref.Key)
	if copyErr := s.copySecretData(ctx, *ref, targetKey, newSecret.Data); copyErr != nil {
		return fmt.Errorf("unable to copy secret data: %w", copyErr)
	}

	return nil
}

// Copies TLS-specific attributes to a secret, that is later mounted as a file, and used in the Fluent Bit configuration
// (since PEM-encoded strings exceed the maximum allowed length of environment variables on some Linux machines).
func (s *syncer) syncTLSFileConfigSecret(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline) error {
	oldSecret, err := k8sutils.GetOrCreateSecret(ctx, s, s.config.TLSFileConfigSecret)
	if err != nil {
		return fmt.Errorf("unable to get tls config secret: %w", err)
	}

	newSecret := oldSecret
	newSecret.Data = make(map[string][]byte)

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
			if err := s.copyFromValueOrSecret(ctx, *tlsConfig.CA, targetKey, newSecret.Data); err != nil {
				return err
			}
		}

		if sharedtypesutils.IsValid(tlsConfig.Cert) && sharedtypesutils.IsValid(tlsConfig.Key) {
			targetCertVariable := fmt.Sprintf("%s-cert.crt", logPipelines[i].Name)
			if err := s.copyFromValueOrSecret(ctx, *tlsConfig.Cert, targetCertVariable, newSecret.Data); err != nil {
				return err
			}

			targetKeyVariable := fmt.Sprintf("%s-key.key", logPipelines[i].Name)
			if err := s.copyFromValueOrSecret(ctx, *tlsConfig.Key, targetKeyVariable, newSecret.Data); err != nil {
				return err
			}

			// Make a best effort replacement of linebreaks in cert/key if present.
			sanitizedCert := bytes.ReplaceAll(newSecret.Data[targetCertVariable], []byte("\\n"), []byte("\n"))
			sanitizedKey := bytes.ReplaceAll(newSecret.Data[targetKeyVariable], []byte("\\n"), []byte("\n"))

			newSecret.Data[targetCertVariable] = sanitizedCert
			newSecret.Data[targetKeyVariable] = sanitizedKey
		}

		if err = controllerutil.SetOwnerReference(&logPipelines[i], &newSecret, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for tls config secret: %w", err)
		}
	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update tls config secret: %w", err)
	}

	return nil
}

func (s *syncer) copyFromValueOrSecret(ctx context.Context, value telemetryv1alpha1.ValueType, targetKey string, target map[string][]byte) error {
	if value.Value != "" {
		target[targetKey] = []byte(value.Value)
		return nil
	}

	if copyErr := s.copySecretData(ctx, *value.ValueFrom.SecretKeyRef, targetKey, target); copyErr != nil {
		return fmt.Errorf("unable to copy secret data: %w", copyErr)
	}

	return nil
}

func (s *syncer) copySecretData(ctx context.Context, sourceRef telemetryv1alpha1.SecretKeyRef, targetKey string, target map[string][]byte) error {
	var source corev1.Secret
	if err := s.Get(ctx, types.NamespacedName{Name: sourceRef.Name, Namespace: sourceRef.Namespace}, &source); err != nil {
		return fmt.Errorf("unable to read secret '%s' from namespace '%s': %w", sourceRef.Name, sourceRef.Namespace, err)
	}

	if val, found := source.Data[sourceRef.Key]; found {
		target[targetKey] = val
		return nil
	}

	return fmt.Errorf("unable to find key '%s' in secret '%s' from namespace '%s'",
		sourceRef.Key,
		sourceRef.Name,
		sourceRef.Namespace)
}

// isLogPipelineDeployable checks if logpipeline is ready to be rendered into the fluentbit configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func isLogPipelineDeployable(allPipelines []telemetryv1alpha1.LogPipeline, logPipeline *telemetryv1alpha1.LogPipeline) bool {
	for i := range allPipelines {
		if allPipelines[i].Name == logPipeline.Name {
			return true
		}
	}

	return false
}
