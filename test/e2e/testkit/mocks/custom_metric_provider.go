package mocks

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var MetricProviderMetricNames = []string{"cpu_temperature_celsius", "hd_errors_total"}

type MetricProvider struct {
	namespace string
}

func NewMetricProvider(namespace string) *MetricProvider {
	return &MetricProvider{
		namespace: namespace,
	}
}

func (mp *MetricProvider) K8sObject() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-metrics",
			Namespace: mp.namespace,
			Annotations: map[string]string{
				"prometheus.io/path":   "/metrics",
				"prometheus.io/port":   "8080",
				"prometheus.io/scrape": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "sample-metrics",
					Image: "eu.gcr.io/kyma-project/develop/monitoring-custom-metrics:e56d9645",
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
				},
			},
		},
	}
}
