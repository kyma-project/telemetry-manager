package k8s

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

type LogPipeline struct {
	persistent bool

	name              string
	secretKeyRef      *telemetryv1alpha1.SecretKeyRef
	systemNamespaces  bool
	includeNamespaces []string
	excludeNamespaces []string
	includeContainers []string
	excludeContainers []string
	keepAnnotations   bool
	dropLabels        bool
	output            telemetryv1alpha1.Output
	filters           []telemetryv1alpha1.Filter
}

func NewLogPipeline(name string) *LogPipeline {
	return &LogPipeline{
		name: name,
	}
}

func (p *LogPipeline) Name() string {
	return p.name
}

func (p *LogPipeline) WithSecretKeyRef(secretKeyRef *telemetryv1alpha1.SecretKeyRef) *LogPipeline {
	p.secretKeyRef = secretKeyRef
	return p
}

func (p *LogPipeline) WithSystemNamespaces(enable bool) *LogPipeline {
	p.systemNamespaces = enable
	return p
}

func (p *LogPipeline) WithIncludeNamespaces(namespaces []string) *LogPipeline {
	p.includeNamespaces = namespaces
	return p
}

func (p *LogPipeline) WithExcludeNamespaces(namespaces []string) *LogPipeline {
	p.excludeNamespaces = namespaces
	return p
}

func (p *LogPipeline) WithIncludeContainers(names []string) *LogPipeline {
	p.includeContainers = names
	return p
}

func (p *LogPipeline) WithExcludeContainers(names []string) *LogPipeline {
	p.excludeContainers = names
	return p
}

func (p *LogPipeline) KeepAnnotations(enable bool) *LogPipeline {
	p.keepAnnotations = enable
	return p
}

func (p *LogPipeline) DropLabels(enable bool) *LogPipeline {
	p.dropLabels = enable
	return p
}

func (p *LogPipeline) WithStdout() *LogPipeline {
	p.output = telemetryv1alpha1.Output{
		Custom: "Name stdout",
	}
	return p
}

func (p *LogPipeline) WithHTTPOutput() *LogPipeline {
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

func (p *LogPipeline) WithTLS(certs tls.Certs) *LogPipeline {
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

func (p *LogPipeline) WithCustomOutput(host string) *LogPipeline {
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

func (p *LogPipeline) WithFilter(filter string) *LogPipeline {
	p.filters = append(p.filters, telemetryv1alpha1.Filter{
		Custom: filter,
	})
	return p
}

func (p *LogPipeline) Persistent(persistent bool) *LogPipeline {
	p.persistent = persistent

	return p
}

func (p *LogPipeline) K8sObject() *telemetryv1alpha1.LogPipeline {
	var labels Labels
	if p.persistent {
		labels = PersistentLabel
	}

	return &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.name,
			Labels: labels,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System:  p.systemNamespaces,
						Include: p.includeNamespaces,
						Exclude: p.excludeNamespaces,
					},
					Containers: telemetryv1alpha1.InputContainers{
						Include: p.includeContainers,
						Exclude: p.excludeContainers,
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
