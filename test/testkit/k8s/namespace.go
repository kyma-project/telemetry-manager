package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamepaceOption func(*Namespace)

type Namespace struct {
	name   string
	labels map[string]string
}

func NewNamespace(name string, opts ...NamepaceOption) *Namespace {
	namespace := &Namespace{
		name: name,
	}

	for _, opt := range opts {
		opt(namespace)
	}

	return namespace
}

func WithIstioInjection() func(*Namespace) {
	return func(n *Namespace) {
		if n.labels == nil {
			n.labels = make(map[string]string)
		}
		n.labels["istio-injection"] = "enabled"
	}
}

func (n *Namespace) Name() string {
	return n.name
}

func (n *Namespace) K8sObject() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   n.name,
			Labels: n.labels,
		},
	}
}
