package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

type Telemetry struct {
	Name       string
	Namespace  string
	persistent bool
}

func NewTelemetry(name, namespace string) *Telemetry {
	return &Telemetry{
		Name:      name,
		Namespace: namespace,
	}
}

func (s *Telemetry) K8sObject() *operatorv1alpha1.Telemetry {
	var labels Labels
	if s.persistent {
		labels = PersistentLabel
	}

	return &operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    labels,
		},
	}
}

func (s *Telemetry) Persistent(persistent bool) *Telemetry {
	s.persistent = persistent

	return s
}
