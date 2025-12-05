package objects

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigMap struct {
	name      string
	namespace string
	labels    map[string]string
	data      map[string]string
}

func NewConfigMap(name, namespace string) *ConfigMap {
	return &ConfigMap{
		name:      name,
		namespace: namespace,
		labels:    make(map[string]string),
		data:      make(map[string]string),
	}
}

func (d *ConfigMap) WithLabel(key, value string) *ConfigMap {
	d.labels[key] = value
	return d
}

func (d *ConfigMap) WithData(data map[string]string) *ConfigMap {
	maps.Copy(d.data, data)
	return d
}

func (d *ConfigMap) K8sObject() *corev1.ConfigMap {
	labels := d.labels
	maps.Copy(labels, PersistentLabel)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.name,
			Namespace: d.namespace,
			Labels:    d.labels,
		},
		Data: d.data,
	}
}
