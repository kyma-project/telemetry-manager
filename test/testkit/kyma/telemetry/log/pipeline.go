package log

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

type Pipeline struct {
	persistent bool

	name             string
	secretKeyRef     *telemetryv1alpha1.SecretKeyRef
	excludeContainer []string
	includeContainer []string
	keepAnnotations  bool
	dropLabels       bool
	output           telemetryv1alpha1.Output
	filters          []telemetryv1alpha1.Filter
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name: name,
	}
}

func (p *Pipeline) Name() string {
	return p.name
}

func (p *Pipeline) WithSecretKeyRef(secretKeyRef *telemetryv1alpha1.SecretKeyRef) *Pipeline {
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
	p.output = telemetryv1alpha1.Output{
		Custom: "Name stdout",
	}
	return p
}

func (p *Pipeline) WithHTTPOutput() *Pipeline {
	p.output = telemetryv1alpha1.Output{
		HTTP: &telemetryv1alpha1.HTTPOutput{
			Dedot: true,
			Host: telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: p.secretKeyRef,
				},
			},
			Port:   "9880",
			URI:    "/",
			Format: "json",
			TLSConfig: telemetryv1alpha1.TLSConfig{
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

	p.output.HTTP.TLSConfig = telemetryv1alpha1.TLSConfig{
		Disabled:                  false,
		SkipCertificateValidation: false,
		CA: &telemetryv1alpha1.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetryv1alpha1.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetryv1alpha1.ValueType{
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
	p.output = telemetryv1alpha1.Output{
		Custom: customOutput,
	}
	return p
}

func (p *Pipeline) WithFilter(filter string) *Pipeline {
	p.filters = append(p.filters, telemetryv1alpha1.Filter{
		Custom: filter,
	})
	return p
}

func (p *Pipeline) Persistent(persistent bool) *Pipeline {
	p.persistent = persistent

	return p
}

func (p *Pipeline) K8sObject() *telemetryv1alpha1.LogPipeline {
	var labels kitk8s.Labels
	if p.persistent {
		labels = kitk8s.PersistentLabel
	}

	return &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.name,
			Labels: labels,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Containers: telemetryv1alpha1.InputContainers{
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
