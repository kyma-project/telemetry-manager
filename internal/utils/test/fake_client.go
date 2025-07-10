package test

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewFakeClientWrapper wraps the fake.ClientBuilder to be instantiated with a kube-system namespace
func NewFakeClientWrapper() *fake.ClientBuilder {
	kubeSystemNamespace := corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: "kube-system",
		},
	}
	return fake.NewClientBuilder().WithObjects(&kubeSystemNamespace)
}
