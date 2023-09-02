package trace

import (
	"fmt"

	"github.com/google/uuid"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

const version = "1.0.0"

type PipelineOption = func(pipeline telemetry.TracePipeline)

type Pipeline struct {
	name         string
	secretKeyRef *telemetry.SecretKeyRef
	persistent   bool
	id           string
}

func NewPipeline(name string, secretKeyRef *telemetry.SecretKeyRef) *Pipeline {
	return &Pipeline{
		name:         name,
		secretKeyRef: secretKeyRef,
		id:           uuid.New().String(),
	}
}

func (p *Pipeline) Name() string {
	if p.persistent {
		return p.name
	}

	return fmt.Sprintf("%s-%s", p.name, p.id)
}

func (p *Pipeline) K8sObject(opts ...PipelineOption) *telemetry.TracePipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
	}
	labels.Version(version)

	pipeline := telemetry.TracePipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.Name(),
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

	for _, opt := range opts {
		opt(pipeline)
	}

	return &pipeline
}

func (p *Pipeline) Persistent(persistent bool) *Pipeline {
	p.persistent = persistent

	return p
}
