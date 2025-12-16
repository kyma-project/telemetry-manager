package otelcollector

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SpecTemplate struct {
	Pod      *corev1.PodTemplateSpec
	Metadata *metav1.ObjectMeta
}
