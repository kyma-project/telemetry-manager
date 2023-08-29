package fluentD

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMap struct {
	name      string
	namespace string
}

func NewConfigMap(name, namespace string) *ConfigMap {
	return &ConfigMap{
		name:      name,
		namespace: namespace,
	}
}

const configTemplateFluentd = `<source>
  @type http
  port 9880
  bind 0.0.0.0
  body_size_limit 32m
  add_http_headers true
  <parse>
    @type json
  </parse>
</source>
<match **>
  @type forward
  send_timeout 60s
  recover_wait 10s
  hard_timeout 60s
  flush_interval 1s

  <server>
    name otlp
    host 127.0.0.1
    port 8006
    weight 60
  </server>
  
</match>`

func (cm *ConfigMap) Name() string {
	return cm.name
}

func (cm *ConfigMap) K8sObject() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.name,
			Namespace: cm.namespace,
		},
		Data: map[string]string{"fluent.conf": configTemplateFluentd},
	}
}
