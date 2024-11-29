package fluentbit

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
)

type Config struct {
	DaemonSet           types.NamespacedName
	SectionsConfigMap   types.NamespacedName
	FilesConfigMap      types.NamespacedName
	LuaConfigMap        types.NamespacedName
	ParsersConfigMap    types.NamespacedName
	EnvConfigSecret     types.NamespacedName
	TLSFileConfigSecret types.NamespacedName
	PipelineDefaults    builder.PipelineDefaults
	Overrides           overrides.Config
	DaemonSetConfig     fluentbit.DaemonSetConfig
	RestConfig          rest.Config
}

type ConfigOption func(*Config)

func WithFluentBitImage(image string) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.FluentBitImage = image
	}
}

func WithExporterImage(image string) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.ExporterImage = image
	}
}

func WithPriorityClassName(name string) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.PriorityClassName = name
	}
}

func WithCPULimit(limit resource.Quantity) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.CPULimit = limit
	}
}

func WithMemoryLimit(limit resource.Quantity) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.MemoryLimit = limit
	}
}

func WithCPURequest(request resource.Quantity) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.CPURequest = request
	}
}

func WithMemoryRequest(request resource.Quantity) ConfigOption {
	return func(c *Config) {
		c.DaemonSetConfig.MemoryRequest = request
	}
}

func NewConfig(baseName string, namespace string, options ...ConfigOption) Config {
	config := Config{
		SectionsConfigMap:   types.NamespacedName{Name: baseName + "-sections", Namespace: namespace},
		FilesConfigMap:      types.NamespacedName{Name: baseName + "-files", Namespace: namespace},
		LuaConfigMap:        types.NamespacedName{Name: baseName + "-luascripts", Namespace: namespace},
		ParsersConfigMap:    types.NamespacedName{Name: baseName + "-parsers", Namespace: namespace},
		EnvConfigSecret:     types.NamespacedName{Name: baseName + "-env", Namespace: namespace},
		TLSFileConfigSecret: types.NamespacedName{Name: baseName + "-output-tls-config", Namespace: namespace},
		DaemonSet:           types.NamespacedName{Name: baseName, Namespace: namespace},
	}

	for _, opt := range options {
		opt(&config)
	}

	return config
}
