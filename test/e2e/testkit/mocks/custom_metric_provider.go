package mocks

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var CustomMetricNames = []string{"cpu_temperature_celsius", "hd_errors_total"}

// CustomMetricProvider represents a workload that exposes dummy metrics in the Prometheus exposition format
type CustomMetricProvider struct {
	namespace string
}

func NewCustomMetricProvider(namespace string) *CustomMetricProvider {
	return &CustomMetricProvider{
		namespace: namespace,
	}
}

func (mp *CustomMetricProvider) K8sObject() *corev1.Pod {
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
					Image: "ghcr.io/skhalash/examples/monitoring-custom-metrics:3d41736",
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
