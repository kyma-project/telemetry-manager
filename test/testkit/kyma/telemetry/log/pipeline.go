package log

import (
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Pipeline struct {
	name string
}

type HTTPPipeline struct {
	name             string
	secretKeyRef     *telemetry.SecretKeyRef
	excludeContainer []string
	includeContainer []string
	keepAnnotations  bool
	dropLabels       bool
}

func (p *HTTPPipeline) Name() string {
	return p.name
}

func NewHTTPPipeline(name string, secretKeyRef *telemetry.SecretKeyRef) *HTTPPipeline {
	return &HTTPPipeline{
		name:         name,
		secretKeyRef: secretKeyRef,
	}
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name: name,
	}
}

func (p *HTTPPipeline) WithIncludeContainer(names []string) *HTTPPipeline {
	p.includeContainer = names
	return p
}

func (p *HTTPPipeline) WithExcludeContainer(names []string) *HTTPPipeline {
	p.excludeContainer = names
	return p
}

func (p *HTTPPipeline) KeepAnnotations(enable bool) *HTTPPipeline {
	p.keepAnnotations = enable
	return p
}

func (p *HTTPPipeline) DropLabels(enable bool) *HTTPPipeline {
	p.dropLabels = enable
	return p
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

func (p *HTTPPipeline) K8sObjectHTTP() *telemetry.LogPipeline {

	return &telemetry.LogPipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name: p.name,
		},
		Spec: telemetry.LogPipelineSpec{
			Input: telemetry.Input{
				Application: telemetry.ApplicationInput{
					Containers: telemetry.InputContainers{
						Exclude: p.excludeContainer,
						Include: p.includeContainer,
					},
					KeepAnnotations: p.keepAnnotations,
					DropLabels:      p.dropLabels,
				},
			},
			Output: telemetry.Output{
				HTTP: &telemetry.HTTPOutput{
					Dedot: true,
					Host: telemetry.ValueType{
						ValueFrom: &telemetry.ValueFromSource{
							SecretKeyRef: p.secretKeyRef,
						},
					},
					Port:   "9880",
					URI:    "/",
					Format: "json",
					TLSConfig: telemetry.TLSConfig{
						Disabled:                  true,
						SkipCertificateValidation: true,
					},
				},
			},
		},
	}
}
