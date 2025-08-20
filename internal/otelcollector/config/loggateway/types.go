package loggateway

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"

type Config struct {
	common.Base `yaml:",inline"`

	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Exporters  map[string]any `yaml:"exporters"`
}

// type Receivers struct {
// 	OTLP common.OTLPReceiver `yaml:"otlp"`
// }

// type Processors struct {
// 	common.BaseProcessors `yaml:",inline"`

// 	// OTel Collector components with static IDs
// 	SetObsTimeIfZero        *common.TransformProcessor         `yaml:"transform/set-observed-time-if-zero,omitempty"`
// 	K8sAttributes           *common.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
// 	IstioNoiseFilter        *common.IstioNoiseFilterProcessor  `yaml:"istio_noise_filter,omitempty"`
// 	InsertClusterAttributes *common.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
// 	ResolveServiceName      *common.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
// 	DropKymaAttributes      *common.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
// 	DropIfInputSourceOTLP   *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
// 	IstioEnrichment         *IstioEnrichmentProcessor          `yaml:"istio_enrichment,omitempty"`

// 	// OTel Collector components with dynamic IDs that are pipeline name based
// 	Dynamic map[string]any `yaml:",inline,omitempty"`
// }

type FilterProcessor struct {
	Logs FilterProcessorLogs `yaml:"logs"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
