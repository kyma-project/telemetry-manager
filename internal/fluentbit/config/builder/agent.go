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

func NewFluentBitConfigBuilder(client client.Reader, builderConfig BuilderConfig) *AgentConfigBuilder {
	return &AgentConfigBuilder{client, builderConfig}
}

type AgentConfigBuilder struct {
	client.Reader
	BuilderConfig
}

func (b *AgentConfigBuilder) Build(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, reconcilablePipelines []telemetryv1alpha1.LogPipeline) (*FluentBitConfig, error) {
	sectionsConfigMapKey := pipeline.Name + ".conf"
	sectionsConfigMapContent, err := BuildFluentBitSectionsConfig(pipeline, b.BuilderConfig)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build section: %w", err)
	}

	envConfigSecret, err := b.BuildEnvConfigSecret(ctx, reconcilablePipelines)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build env config: %w", err)
	}

	tlsConfigSecret, err := b.BuildTLSFileConfigSecret(ctx, reconcilablePipelines)
	if err != nil {
		return &FluentBitConfig{}, fmt.Errorf("unable to build tls secret: %w", err)
	}

	return &FluentBitConfig{
		SectionsConfig:  SectionsConfig{sectionsConfigMapKey, sectionsConfigMapContent},
		FilesConfig:     BuildFluentBitFilesConfig(pipeline),
		EnvConfigSecret: envConfigSecret,
		TLSConfigSecret: tlsConfigSecret,
	}, nil

}
