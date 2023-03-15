package logpipeline

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	utils "github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

	changed := false
	cmKey := pipeline.Name + ".conf"
	if pipeline.DeletionTimestamp != nil {
		if cm.Data != nil {
			delete(cm.Data, cmKey)
			changed = true
		}
	} else {
		newConfig, err := builder.BuildFluentBitConfig(pipeline, s.config.PipelineDefaults)
		if err != nil {
			return fmt.Errorf("unable to build section: %w", err)
		}
		if cm.Data == nil {
			cm.Data = map[string]string{cmKey: newConfig}
			changed = true
		} else if oldConfig, hasKey := cm.Data[cmKey]; !hasKey || oldConfig != newConfig {
			cm.Data[cmKey] = newConfig
			changed = true
		}
	}

	if !changed {
		return nil
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

	changed := false
	for _, file := range pipeline.Spec.Files {
		if pipeline.DeletionTimestamp != nil {
			if _, hasKey := cm.Data[file.Name]; hasKey {
				delete(cm.Data, file.Name)
				changed = true
			}
		} else {
			if cm.Data == nil {
				cm.Data = map[string]string{file.Name: file.Content}
				changed = true
			} else if oldContent, hasKey := cm.Data[file.Name]; !hasKey || oldContent != file.Content {
				cm.Data[file.Name] = file.Content
				changed = true
			}
		}
	}

	if !changed {
		return nil
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
		if !logPipelines.Items[i].DeletionTimestamp.IsZero() {
			continue
		}

		refSecretData, copyErr := secretref.FetchReferencedSecretData(ctx, s.Client, &logPipelines.Items[i])
		if copyErr != nil {
			return fmt.Errorf("unable to copy secret data: %w", copyErr)
		}

		for k, v := range refSecretData {
			newSecret.Data[k] = v
		}
	}

	changed := secretDataEqual(oldSecret.Data, newSecret.Data)
	if !changed {
		return nil
	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update env secret: %w", err)
	}
	return nil
}

func secretDataEqual(oldSecret, newSecret map[string][]byte) bool {
	if len(newSecret) != len(oldSecret) {
		return true
	}
	for k, newSecretVal := range newSecret {
		if oldSecretVal, ok := oldSecret[k]; !ok || !bytes.Equal(newSecretVal, oldSecretVal) {
			return true
		}
	}
	return false
}
