//go:build e2e

package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Namespace struct {
	name string
}

func NewNamespace(name string) *Namespace {
	return &Namespace{
		name: name,
	}
}

func (n *Namespace) Name() string {
	return n.name
}

func (n *Namespace) K8sObject() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: n.name,
		},
	}
}
