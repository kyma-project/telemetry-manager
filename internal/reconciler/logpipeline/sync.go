package logpipeline

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/secretref"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	utils "github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

type syncer struct {
	client.Client
	config Config
}

func (s *syncer) syncFluentBitConfig(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {

	if err := s.syncSectionsConfigMap(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to sync sections: %v", err)
	}

	if err := s.syncFilesConfigMap(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to sync mounted files: %v", err)
	}

	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := s.List(ctx, &allPipelines); err != nil {
		return fmt.Errorf("failed to get all log pipelines while syncing Fluent Bit ConfigMaps: %v", err)
	}
	if err := s.syncReferencedSecrets(ctx, &allPipelines); err != nil {
		return fmt.Errorf("failed to sync referenced secrets: %v", err)
	}

	return nil
}

func (s *syncer) syncSectionsConfigMap(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	cm, err := utils.GetOrCreateConfigMap(ctx, s, s.config.SectionsConfigMap)
	if err != nil {
		return fmt.Errorf("unable to get section configmap: %w", err)
	}

	cmKey := pipeline.Name + ".conf"
	deployable, err := s.isLogPipelineDeployable(ctx, pipeline)
	if err != nil {
		return err
	}

	if !deployable {
		delete(cm.Data, cmKey)
	} else {
		newConfig, err := builder.BuildFluentBitConfig(pipeline, s.config.PipelineDefaults)
		if err != nil {
			return fmt.Errorf("unable to build section: %w", err)
		}
		if cm.Data == nil {
			cm.Data = map[string]string{cmKey: newConfig}
		} else if oldConfig, hasKey := cm.Data[cmKey]; !hasKey || oldConfig != newConfig {
			cm.Data[cmKey] = newConfig
		}

		if err = controllerutil.SetOwnerReference(pipeline, &cm, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for section configmap: %w", err)
		}
	}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update section configmap: %w", err)
	}
	return nil
}

func (s *syncer) syncFilesConfigMap(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	cm, err := utils.GetOrCreateConfigMap(ctx, s, s.config.FilesConfigMap)
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

func (s *syncer) syncReferencedSecrets(ctx context.Context, logPipelines *telemetryv1alpha1.LogPipelineList) error {
	oldSecret, err := utils.GetOrCreateSecret(ctx, s, s.config.EnvSecret)
	if err != nil {
		return fmt.Errorf("unable to get env secret: %w", err)
	}

	newSecret := oldSecret
	newSecret.Data = make(map[string][]byte)

	for i := range logPipelines.Items {
		logPipeline := logPipelines.Items[i]
		if !logPipeline.DeletionTimestamp.IsZero() {
			continue
		}

		for _, ref := range logPipeline.GetSecretRefs() {
			targetKey := envvar.FormatEnvVarName(logPipeline.Name, ref.Namespace, ref.Name, ref.Key)
			if copyErr := s.copySecretData(ctx, ref, targetKey, newSecret.Data); copyErr != nil {
				return fmt.Errorf("unable to copy secret data: %w", copyErr)
			}
		}

		for _, ref := range logPipeline.Spec.Variables {
			if ref.ValueFrom.IsSecretKeyRef() {
				if copyErr := s.copySecretData(ctx, *ref.ValueFrom.SecretKeyRef, ref.Name, newSecret.Data); copyErr != nil {
					return fmt.Errorf("unable to copy secret data: %w", copyErr)
				}
			}
		}

		if err = controllerutil.SetOwnerReference(&logPipeline, &newSecret, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for files configmap: %w", err)
		}
	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update env secret: %w", err)
	}
	return nil
}

func (s *syncer) copySecretData(ctx context.Context, sourceRef telemetryv1alpha1.SecretKeyRef, targetKey string, target map[string][]byte) error {
	var source corev1.Secret
	if err := s.Get(ctx, sourceRef.NamespacedName(), &source); err != nil {
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
func (s *syncer) isLogPipelineDeployable(ctx context.Context, logPipeline *telemetryv1alpha1.LogPipeline) (bool, error) {

	if !logPipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	if secretref.ReferencesNonExistentSecret(ctx, s.Client, logPipeline) {
		return false, nil
	}
	return true, nil
}
