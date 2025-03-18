package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	OTLP config.OTLPReceiver `yaml:"otlp"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	SetObsTimeIfZero        *log.TransformProcessor        `yaml:"transform/set-observed-time-if-zero,omitempty"`
	K8sAttributes           *config.K8sAttributesProcessor `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes *config.ResourceProcessor      `yaml:"resource/insert-cluster-attributes,omitempty"`
	DropKymaAttributes      *config.ResourceProcessor      `yaml:"resource/drop-kyma-attributes,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
