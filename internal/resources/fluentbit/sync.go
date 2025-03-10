package fluentbit

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

type syncer struct {
	client.Client
	Config    *builder.FluentBitConfig
	namespace string
}

func (s *syncer) syncFluentBitConfig(ctx context.Context) error {
	log := logf.FromContext(ctx)

	if s.Config == nil {
		return fmt.Errorf("fluent bit config not defined")
	}

	if err := s.syncSectionsConfigMap(ctx); err != nil {
		return fmt.Errorf("failed to sync sections: %w", err)
	}

	if err := s.syncFilesConfigMap(ctx); err != nil {
		return fmt.Errorf("failed to sync mounted files: %w", err)
	}

	if err := s.syncEnvConfigSecret(ctx); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("referenced secret not found: %v", err))
			return nil
		}

		return err
	}

	if err := s.syncTLSFileConfigSecret(ctx); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info(fmt.Sprintf("referenced tls config secret not found: %v", err))
			return nil
		}

		return err
	}

	return nil
}

func (s *syncer) syncSectionsConfigMap(ctx context.Context) error {
	sectionsConfigMapName := types.NamespacedName{Name: fbSectionsConfigMapName, Namespace: s.namespace}

	cm, err := k8sutils.GetOrCreateConfigMap(ctx, s, sectionsConfigMapName, Labels())
	if err != nil {
		return fmt.Errorf("unable to get section configmap: %w", err)
	}

	//cmKey := pipeline.Name + ".conf"
	//
	//if !isLogPipelineDeployable(deployablePipelines, pipeline) || !pipeline.DeletionTimestamp.IsZero() {
	//	delete(cm.Data, cmKey)
	//} else {
	//	builderConfig := builder.BuilderConfig{
	//		PipelineDefaults: s.Config.PipelineDefaults,
	//		CollectAgentLogs: s.Config.Overrides.Logging.CollectAgentLogs,
	//	}
	//
	//	newConfig, err := builder.BuildFluentBitSectionsConfig(pipeline, builderConfig)
	//	if err != nil {
	//		return fmt.Errorf("unable to build section: %w", err)
	//	}

	cmKey := s.Config.SectionsConfig.Key
	newConfig := s.Config.SectionsConfig.Value

	if cm.Data == nil {
		cm.Data = map[string]string{cmKey: newConfig}
	} else if oldConfig, hasKey := cm.Data[cmKey]; !hasKey || oldConfig != newConfig {
		cm.Data[cmKey] = newConfig
	}
	//}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update section configmap: %w", err)
	}

	return nil
}

func (s *syncer) syncFilesConfigMap(ctx context.Context) error {
	filesConfigMapName := types.NamespacedName{Name: fbFilesConfigMapName, Namespace: s.namespace}

	cm, err := k8sutils.GetOrCreateConfigMap(ctx, s, filesConfigMapName, Labels())
	if err != nil {
		return fmt.Errorf("unable to get files configmap: %w", err)
	}

	for name, content := range s.Config.FilesConfig {
		if cm.Data == nil {
			cm.Data = map[string]string{name: content}
		} else if oldContent, hasKey := cm.Data[name]; !hasKey || oldContent != content {
			cm.Data[name] = content
		}
	}

	//if pipeline.DeletionTimestamp.IsZero() {
	//	if err = controllerutil.SetOwnerReference(pipeline, &cm, s.Scheme()); err != nil {
	//		return fmt.Errorf("unable to set owner reference for files configmap: %w", err)
	//	}
	//}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update files configmap: %w", err)
	}

	return nil
}

// Copies HTTP-specific attributes and user-provided variables to a secret that is later used for providing environment variables to the Fluent Bit configuration.
func (s *syncer) syncEnvConfigSecret(ctx context.Context) error {
	envConfigSecretName := types.NamespacedName{Name: fbEnvConfigSecretName, Namespace: s.namespace}

	oldSecret, err := k8sutils.GetOrCreateSecret(ctx, s, envConfigSecretName, Labels())
	if err != nil {
		return fmt.Errorf("unable to get env secret: %w", err)
	}

	newSecret := oldSecret
	newSecret.Data = make(map[string][]byte)

	for key, value := range s.Config.EnvConfigSecret {
		newSecret.Data[key] = value
	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update env secret: %w", err)
	}

	return nil
}

// Copies TLS-specific attributes to a secret, that is later mounted as a file, and used in the Fluent Bit configuration
// (since PEM-encoded strings exceed the maximum allowed length of environment variables on some Linux machines).
func (s *syncer) syncTLSFileConfigSecret(ctx context.Context) error {
	tlsFileConfigSecretName := types.NamespacedName{Name: fbTLSFileConfigSecretName, Namespace: s.namespace}

	oldSecret, err := k8sutils.GetOrCreateSecret(ctx, s, tlsFileConfigSecretName, Labels())
	if err != nil {
		return fmt.Errorf("unable to get tls config secret: %w", err)
	}

	newSecret := oldSecret
	newSecret.Data = make(map[string][]byte)

	for key, value := range s.Config.TLSConfigSecret {
		newSecret.Data[key] = value

	}

	if err = s.Update(ctx, &newSecret); err != nil {
		return fmt.Errorf("unable to update tls config secret: %w", err)
	}

	return nil
}
