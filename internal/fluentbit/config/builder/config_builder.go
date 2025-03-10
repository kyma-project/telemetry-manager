package builder

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type FluentBitConfig struct {
	SectionsConfig  map[string]string
	FilesConfig     map[string]string
	EnvConfigSecret map[string][]byte
	TLSConfigSecret map[string][]byte
}

func (f *FluentBitConfig) addSections(sections map[string]string) {
	maps.Copy(f.SectionsConfig, sections)
}

func (f *FluentBitConfig) addFiles(files map[string]string) {
	maps.Copy(f.FilesConfig, files)
}

func (f *FluentBitConfig) addEnvConfigSecret(envConfigSecret map[string][]byte) {
	maps.Copy(f.EnvConfigSecret, envConfigSecret)
}

func (f *FluentBitConfig) addTLSConfigSecret(tlsConfigSecret map[string][]byte) {
	maps.Copy(f.TLSConfigSecret, tlsConfigSecret)
}

func NewFluentBitConfigBuilder(client client.Reader, builderConfig BuilderConfig) *ConfigBuilder {
	return &ConfigBuilder{client, builderConfig}
}

type ConfigBuilder struct {
	client.Reader
	BuilderConfig
}

func (b *ConfigBuilder) Build(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) (*FluentBitConfig, error) {
	config := FluentBitConfig{}

	for _, pipeline := range allPipelines {
		if !isLogPipelineReconcilable(allPipelines, &pipeline) || !pipeline.DeletionTimestamp.IsZero() {
			continue
		}

		sectionsConfigMapKey := pipeline.Name + ".conf"

		sectionsConfigMapContent, err := BuildFluentBitSectionsConfig(&pipeline, b.BuilderConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to build section: %w", err)
		}

		filesConfig := BuildFluentBitFilesConfig(&pipeline)

		envConfigSecret, err := b.BuildEnvConfigSecret(ctx, allPipelines)
		if err != nil {
			return nil, fmt.Errorf("unable to build env config: %w", err)
		}

		tlsConfigSecret, err := b.BuildTLSFileConfigSecret(ctx, allPipelines)
		if err != nil {
			return nil, fmt.Errorf("unable to build tls secret: %w", err)
		}

		config.addSections(map[string]string{sectionsConfigMapKey: sectionsConfigMapContent})
		config.addFiles(filesConfig)
		config.addEnvConfigSecret(envConfigSecret)
		config.addTLSConfigSecret(tlsConfigSecret)
	}

	return &config, nil
}

// isLogPipelineReconcilable checks if logpipeline is ready to be rendered into the fluentbit configuration.
// A pipeline is reconcilable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func isLogPipelineReconcilable(allPipelines []telemetryv1alpha1.LogPipeline, logPipeline *telemetryv1alpha1.LogPipeline) bool {
	for i := range allPipelines {
		if allPipelines[i].Name == logPipeline.Name {
			return true
		}
	}

	return false
}
