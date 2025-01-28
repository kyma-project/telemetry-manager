package k8s

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMap struct {
	name      string
	namespace string
	data      map[string]string
	labels    map[string]string
}

func NewConfigMap(cfName, ns string) *ConfigMap {
	return &ConfigMap{
		name:      cfName,
		namespace: ns,
		data:      make(map[string]string),
		labels:    make(map[string]string),
	}
}

func (c *ConfigMap) WithData(key, value string) *ConfigMap {
	maps.Copy(c.data, map[string]string{key: value})
	return c
}

func (c *ConfigMap) WithLabel(key, value string) *ConfigMap {
	c.labels[key] = value
	return c
}

func (c *ConfigMap) K8sObject() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.name,
			Namespace: c.namespace,
			Labels:    c.labels,
		},
		Data: c.data,
	}
}
