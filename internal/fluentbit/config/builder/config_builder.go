package builder

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SectionsConfig struct {
	Key   string
	Value string
}

type FluentBitConfig struct {
	SectionsConfig  SectionsConfig
	FilesConfig     map[string]string
	EnvConfigSecret map[string][]byte
	TLSConfigSecret map[string][]byte
}

func NewFluentBitConfigBuilder(client client.Reader, builderConfig BuilderConfig) *ConfigBuilder {
	return &ConfigBuilder{client, builderConfig}
}

type ConfigBuilder struct {
	client.Reader
	BuilderConfig
}

func (b *ConfigBuilder) Build(ctx context.Context, currentPipeline *telemetryv1alpha1.LogPipeline, allPipelines []telemetryv1alpha1.LogPipeline) (*FluentBitConfig, error) {
	sectionsConfigMapKey := currentPipeline.Name + ".conf"
	sectionsConfigMapContent, err := BuildFluentBitSectionsConfig(currentPipeline, b.BuilderConfig)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build section: %w", err)
	}

	envConfigSecret, err := b.BuildEnvConfigSecret(ctx, allPipelines)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build env config: %w", err)
	}

	tlsConfigSecret, err := b.BuildTLSFileConfigSecret(ctx, allPipelines)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build tls secret: %w", err)
	}

	return &FluentBitConfig{
		SectionsConfig:  SectionsConfig{sectionsConfigMapKey, sectionsConfigMapContent},
		FilesConfig:     BuildFluentBitFilesConfig(currentPipeline),
		EnvConfigSecret: envConfigSecret,
		TLSConfigSecret: tlsConfigSecret,
	}, nil

}
