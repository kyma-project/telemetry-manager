//go:build e2e

package mocks

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackendConfigMap struct {
	name      string
	namespace string
	path      string
	pipeline  string
}

func NewBackendConfigMap(name, namespace, pipeline, path string) *BackendConfigMap {
	return &BackendConfigMap{
		name:      name,
		namespace: namespace,
		path:      path,
		pipeline:  pipeline,
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
    {{ PIPELINE_NAME }}:
      receivers:
        - otlp
      exporters:
        - file
        - logging`

func (cm *BackendConfigMap) Name() string {
	return cm.name
}

func (cm *BackendConfigMap) K8sObject() *corev1.ConfigMap {
	config := strings.Replace(configTemplate, "{{ FILEPATH }}", cm.path, 1)
	config = strings.Replace(config, "{{ PIPELINE_NAME }}", cm.pipeline, 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{"config.yaml": config},
	}
}
