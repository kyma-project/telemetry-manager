//go:build e2e

package trace

import (
	"github.com/google/uuid"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
)

type Pipeline struct {
	name         string
	secretKeyRef *telemetry.SecretKeyRef
	persistent   bool
}

func NewPipeline(name string, secretKeyRef *telemetry.SecretKeyRef) *Pipeline {
	return &Pipeline{
		name:         name + uuid.New().String(),
		secretKeyRef: secretKeyRef,
	}
}

func (p *Pipeline) Name() string {
	return p.name
}

func (p *Pipeline) K8sObject() *telemetry.TracePipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
	}

	return &telemetry.TracePipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.name,
			Labels: labels,
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

func (p *Pipeline) Persistent(persistent bool) *Pipeline {
	p.persistent = persistent

	return p
}
