package test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceBuilder is a test utility for building corev1.Service objects.
type ServiceBuilder struct {
	service corev1.Service
}

func NewServiceBuilder() *ServiceBuilder {
	return &ServiceBuilder{
		service: corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "kyma-system",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "test"},
				Ports: []corev1.ServicePort{
					{Port: 80, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	}
}

func (b *ServiceBuilder) WithName(name string) *ServiceBuilder {
	b.service.Name = name
	return b
}

func (b *ServiceBuilder) WithNamespace(namespace string) *ServiceBuilder {
	b.service.Namespace = namespace
	return b
}

func (b *ServiceBuilder) Build() corev1.Service {
	return b.service
}
