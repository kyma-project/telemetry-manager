package builder

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

const (
	defaultInputTag          = "tele"
	defaultMemoryBufferLimit = "10M"
	defaultStorageType       = "filesystem"
	defaultFsBufferLimit     = "1G"
)

type pipelineDefaults struct {
	InputTag          string
	MemoryBufferLimit string
	StorageType       string
	FsBufferLimit     string
}

// FluentBit builder configuration
type builderConfig struct {
	pipelineDefaults

	collectAgentLogs bool
}

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

type ConfigBuilder struct {
	reader client.Reader
	cfg    builderConfig
}

func NewFluentBitConfigBuilder(client client.Reader) *ConfigBuilder {
	return &ConfigBuilder{
		reader: client,
		cfg: builderConfig{
			pipelineDefaults: pipelineDefaults{
				InputTag:          defaultInputTag,
				MemoryBufferLimit: defaultMemoryBufferLimit,
				StorageType:       defaultStorageType,
				FsBufferLimit:     defaultFsBufferLimit,
			},
		},
	}
}

func (b *ConfigBuilder) Build(ctx context.Context, allPipelines []telemetryv1beta1.LogPipeline, clusterName string) (*FluentBitConfig, error) {
	config := FluentBitConfig{
		SectionsConfig:  make(map[string]string),
		FilesConfig:     make(map[string]string),
		EnvConfigSecret: make(map[string][]byte),
		TLSConfigSecret: make(map[string][]byte),
	}

	for _, pipeline := range allPipelines {
		sectionsConfigMapKey := pipeline.Name + ".conf"

		sectionsConfigMapContent, err := buildFluentBitSectionsConfig(&pipeline, b.cfg, clusterName)
		if err != nil {
			return nil, fmt.Errorf("unable to build section: %w", err)
		}

		filesConfig := buildFluentBitFilesConfig(&pipeline)

		envConfigSecret, err := b.buildEnvConfigSecret(ctx, allPipelines)
		if err != nil {
			return nil, fmt.Errorf("unable to build env config: %w", err)
		}

		tlsConfigSecret, err := b.buildTLSFileConfigSecret(ctx, allPipelines)
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
