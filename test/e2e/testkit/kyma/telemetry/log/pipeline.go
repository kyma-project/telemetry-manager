package log

import (
	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pipeline struct {
	name         string
	secretKeyRef *telemetry.SecretKeyRef
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name: name,
	}
}
func (p *Pipeline) K8sObject() *telemetry.LogPipeline {
	return &telemetry.LogPipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: p.name,
		},
		Spec: telemetry.LogPipelineSpec{
			Output: telemetry.Output{
				Custom: "Name               stdout",
			},
		},
	}
}
