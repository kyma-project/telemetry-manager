//go:build e2e

package trace

import (
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Pipeline struct {
	name         string
	secretKeyRef *telemetry.SecretKeyRef
}

func NewPipeline(name string, secretKeyRef *telemetry.SecretKeyRef) *Pipeline {
	return &Pipeline{
		name:         name,
		secretKeyRef: secretKeyRef,
	}
}

func (p *Pipeline) K8sObject() *telemetry.TracePipeline {
	return &telemetry.TracePipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: p.name,
		},
		Spec: telemetry.TracePipelineSpec{
			Output: telemetry.TracePipelineOutput{
				Otlp: &telemetry.OtlpOutput{
					Endpoint: telemetry.ValueType{
						ValueFrom: &telemetry.ValueFromSource{
							SecretKeyRef: p.secretKeyRef,
						},
					},
				},
			},
		},
	}
}
