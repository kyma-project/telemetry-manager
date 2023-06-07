//go:build e2e

package telemetry

import (
	operator "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Telemetry struct {
	name string
}

func NewTelemetry(name string) *Telemetry {
	return &Telemetry{
		name: name,
	}
}
func (t *Telemetry) K8sObject() *operator.Telemetry {
	return &operator.Telemetry{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: t.name,
		},
	}
}
