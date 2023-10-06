package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceOption func(*Namespace)

type Namespace struct {
	persistent bool

	name string
}

func NewNamespace(name string) *Namespace {
	namespace := &Namespace{
		name: name,
	}

	return namespace
}

func (n *Namespace) Persistent(persistent bool) *Namespace {
	n.persistent = persistent
	return n
}

func (n *Namespace) Name() string {
	return n.name
}

func (n *Namespace) K8sObject() *corev1.Namespace {
	var labels Labels
	if n.persistent {
		labels = PersistentLabel
	}

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   n.name,
			Labels: labels,
		},
	}
}
