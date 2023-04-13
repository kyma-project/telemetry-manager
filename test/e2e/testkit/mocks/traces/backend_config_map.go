//go:build e2e

package traces

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackendConfigMap struct {
	name      string
	namespace string
}

func NewBackendConfigMap(name, namespace string) *BackendConfigMap {
	return &BackendConfigMap{
		name:      name,
		namespace: namespace,
	}
}

func (cm *BackendConfigMap) K8sObject() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{
			"config.yaml": `receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
exporters:
  file:
    path: /traces/spans.json
  logging:
    loglevel: debug
service:
  pipelines:
    traces:
      receivers:
      - otlp
      exporters:
      - file
      - logging`,
		},
	}
}
