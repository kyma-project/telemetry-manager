package objects

import (
	"google.golang.org/protobuf/types/known/wrapperspb"
	istiotelemetryv1alpha1 "istio.io/api/telemetry/v1alpha1"
	istiotelemetryclientv1 "istio.io/client-go/pkg/apis/telemetry/v1"
)

type IstioTelemetry struct {
	name               string
	namespace          string
	samplingPercentage *float64
}

func NewIstioTelemetry(name, namespace string) *IstioTelemetry {
	return &IstioTelemetry{
		name:      name,
		namespace: namespace,
	}
}

func (t *IstioTelemetry) WithSamplingPercentage(rate float64) *IstioTelemetry {
	t.samplingPercentage = &rate
	return t
}

func (t *IstioTelemetry) K8sObject() *istiotelemetryclientv1.Telemetry {
	tracing := &istiotelemetryv1alpha1.Tracing{
		Providers: []*istiotelemetryv1alpha1.ProviderRef{{Name: "kyma-traces"}},
	}

	if t.samplingPercentage != nil {
		tracing.RandomSamplingPercentage = wrapperspb.Double(*t.samplingPercentage)
	}

	return &istiotelemetryclientv1.Telemetry{
		Name:      t.name,
		Namespace: t.namespace,
		Spec: istiotelemetryv1alpha1.Telemetry{
			Tracing: []*istiotelemetryv1alpha1.Tracing{tracing},
		},
	}
}
