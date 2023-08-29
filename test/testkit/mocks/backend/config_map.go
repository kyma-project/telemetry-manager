package backend

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SignalType string

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
	SignalTypeLogs    = "logs"
)

type ConfigMap struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
}

func NewConfigMap(name, namespace, path string, signalType SignalType) *ConfigMap {
	return &ConfigMap{
		name:             name,
		namespace:        namespace,
		exportedFilePath: path,
		signalType:       signalType,
	}
}

const metricsAndTracesConfigTemplate = `receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
exporters:
  file:
    path: {{ FILEPATH }}
  logging:
    loglevel: debug
service:
  telemetry:
    logs:
      level: "debug"
  pipelines:
    {{ SIGNAL_TYPE }}:
      receivers:
        - otlp
      exporters:
        - file
        - logging`

const LogConfigTemplate = `receivers:
  fluentforward:
    endpoint: 0.0.0.0:8006
  otlp:
    protocols:
      grpc: {}
      http: {}
exporters:
  file:
    path: {{ FILEPATH }}
  logging:
    loglevel: debug
service:
  telemetry:
    logs:
      level: "info"
  pipelines:
    {{ SIGNAL_TYPE }}:
    logs:
      receivers:
        - otlp
        - fluentforward
      exporters:
        - file
        - logging`

func (cm *ConfigMap) Name() string {
	return cm.name
}

func (cm *ConfigMap) K8sObject() *corev1.ConfigMap {
	var configTemplate string
	if cm.signalType == SignalTypeLogs {
		configTemplate = LogConfigTemplate
	} else {
		configTemplate = metricsAndTracesConfigTemplate
	}
	config := strings.Replace(configTemplate, "{{ FILEPATH }}", cm.exportedFilePath, 1)
	config = strings.Replace(config, "{{ SIGNAL_TYPE }}", string(cm.signalType), 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{"config.yaml": config},
	}
}
