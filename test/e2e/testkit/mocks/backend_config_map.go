//go:build e2e

package mocks

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SignalType string

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
)

type BackendConfigMap struct {
	name             string
	namespace        string
	exportedFilePath string
	signalType       SignalType
}

func NewBackendConfigMap(name, namespace, path string, signalType SignalType) *BackendConfigMap {
	return &BackendConfigMap{
		name:             name,
		namespace:        namespace,
		exportedFilePath: path,
		signalType:       signalType,
	}
}

const configTemplate = `receivers:
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

func (cm *BackendConfigMap) Name() string {
	return cm.name
}

func (cm *BackendConfigMap) K8sObject() *corev1.ConfigMap {
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
