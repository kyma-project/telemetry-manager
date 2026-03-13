package objects

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceAccountOption func(*ServiceAccount)

type ServiceAccount struct {
	name      string
	namespace string
}

func NewServiceAccount(name, namespace string, opts ...ServiceAccountOption) *ServiceAccount {
	sa := &ServiceAccount{
		name:      name,
		namespace: namespace,
	}

	for _, opt := range opts {
		opt(sa)
	}

	return sa
}

func (s *ServiceAccount) Name() string {
	return s.name
}

func (s *ServiceAccount) Namespace() string {
	return s.namespace
}

func (s *ServiceAccount) K8sObject() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
	}
}
