package log

import (
	"strings"

	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetry "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

type Pipeline struct {
	persistent bool

	name             string
	secretKeyRef     *telemetry.SecretKeyRef
	excludeContainer []string
	includeContainer []string
	keepAnnotations  bool
	dropLabels       bool
	output           telemetry.Output
	filters          []telemetry.Filter
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name: name,
	}
}

func (p *Pipeline) Name() string {
	return p.name
}

func (p *Pipeline) WithSecretKeyRef(secretKeyRef *telemetry.SecretKeyRef) *Pipeline {
	p.secretKeyRef = secretKeyRef
	return p
}

func (p *Pipeline) WithIncludeContainer(names []string) *Pipeline {
	p.includeContainer = names
	return p
}

func (p *Pipeline) WithExcludeContainer(names []string) *Pipeline {
	p.excludeContainer = names
	return p
}

func (p *Pipeline) KeepAnnotations(enable bool) *Pipeline {
	p.keepAnnotations = enable
	return p
}

func (p *Pipeline) DropLabels(enable bool) *Pipeline {
	p.dropLabels = enable
	return p
}

func (p *Pipeline) WithStdout() *Pipeline {
	p.output = telemetry.Output{
		Custom: "Name stdout",
	}
	return p
}

func (p *Pipeline) WithHTTPOutput() *Pipeline {
	p.output = telemetry.Output{
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
	}
	return p
}

func (p *Pipeline) WithTLS(certs tls.Certs) *Pipeline {
	if !p.output.IsHTTPDefined() {
		return p
	}

	p.output.HTTP.TLSConfig = telemetry.TLSConfig{
		Disabled:                  false,
		SkipCertificateValidation: false,
		CA: &telemetry.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetry.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetry.ValueType{
			Value: certs.ClientKeyPem.String(),
		},
	}

	return p
}

func (p *Pipeline) WithCustomOutput(host string) *Pipeline {
	const customOutputTemplate = `
	name   http
	port   9880
	host   {{ HOST }}
	format json`
	customOutput := strings.Replace(customOutputTemplate, "{{ HOST }}", host, 1)
	p.output = telemetry.Output{
		Custom: customOutput,
	}
	return p
}

func (p *Pipeline) WithFilter(filter string) *Pipeline {
	p.filters = append(p.filters, telemetry.Filter{
		Custom: filter,
	})
	return p
}

func (p *Pipeline) Persistent(persistent bool) *Pipeline {
	p.persistent = persistent

	return p
}

func (p *Pipeline) K8sObject() *telemetry.LogPipeline {
	var labels k8s.Labels
	if p.persistent {
		labels = k8s.PersistentLabel
	}

	return &telemetry.LogPipeline{
		ObjectMeta: k8smeta.ObjectMeta{
			Name:   p.name,
			Labels: labels,
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
			Output:  p.output,
			Filters: p.filters,
		},
	}
}
