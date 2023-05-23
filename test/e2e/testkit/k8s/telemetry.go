package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
)

type Telemetry struct {
	Name string
}

func NewTelemetry(name string) *Telemetry {
	return &Telemetry{
		Name: name,
	}
}

func (s *Telemetry) K8sObject() *operatorv1alpha1.Telemetry {
	return &operatorv1alpha1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Name,
		},
	}
}
