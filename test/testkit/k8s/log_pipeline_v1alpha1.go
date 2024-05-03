//nolint:dupl //There is duplication between logPipelineV1Beta1 and logPipelineV1Alpha1, but we need them as separate builders because they are using different API versions
package k8s

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/testkit/tlsgen"
)

type logPipelineV1Alpha1 struct {
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

func NewLogPipelineV1Alpha1(name string) *logPipelineV1Alpha1 {
	return &logPipelineV1Alpha1{
		name: name,
	}
}

func (p *logPipelineV1Alpha1) Name() string {
	return p.name
}

func (p *logPipelineV1Alpha1) WithSecretKeyRef(secretKeyRef *telemetryv1alpha1.SecretKeyRef) *logPipelineV1Alpha1 {
	p.secretKeyRef = secretKeyRef
	return p
}

func (p *logPipelineV1Alpha1) WithSystemNamespaces(enable bool) *logPipelineV1Alpha1 {
	p.systemNamespaces = enable
	return p
}

func (p *logPipelineV1Alpha1) WithIncludeNamespaces(namespaces []string) *logPipelineV1Alpha1 {
	p.includeNamespaces = namespaces
	return p
}

func (p *logPipelineV1Alpha1) WithExcludeNamespaces(namespaces []string) *logPipelineV1Alpha1 {
	p.excludeNamespaces = namespaces
	return p
}

func (p *logPipelineV1Alpha1) WithIncludeContainers(names []string) *logPipelineV1Alpha1 {
	p.includeContainers = names
	return p
}

func (p *logPipelineV1Alpha1) WithExcludeContainers(names []string) *logPipelineV1Alpha1 {
	p.excludeContainers = names
	return p
}

func (p *logPipelineV1Alpha1) KeepAnnotations(enable bool) *logPipelineV1Alpha1 {
	p.keepAnnotations = enable
	return p
}

func (p *logPipelineV1Alpha1) DropLabels(enable bool) *logPipelineV1Alpha1 {
	p.dropLabels = enable
	return p
}

func (p *logPipelineV1Alpha1) WithStdout() *logPipelineV1Alpha1 {
	p.output = telemetryv1alpha1.Output{
		Custom: "Name stdout",
	}
	return p
}

func (p *logPipelineV1Alpha1) WithHTTPOutput() *logPipelineV1Alpha1 {
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

func (p *logPipelineV1Alpha1) WithTLS(certs tlsgen.ClientCerts) *logPipelineV1Alpha1 {
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

func (p *logPipelineV1Alpha1) WithCustomOutput(host string) *logPipelineV1Alpha1 {
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

func (p *logPipelineV1Alpha1) WithLokiOutput() *logPipelineV1Alpha1 {
	p.output = telemetryv1alpha1.Output{
		Loki: &telemetryv1alpha1.LokiOutput{
			URL: telemetryv1alpha1.ValueType{
				Value: "http://logging-loki:3100/loki/api/v1/push",
			},
		},
	}

	return p
}

func (p *logPipelineV1Alpha1) WithFilter(filter string) *logPipelineV1Alpha1 {
	p.filters = append(p.filters, telemetryv1alpha1.Filter{
		Custom: filter,
	})
	return p
}

func (p *logPipelineV1Alpha1) Persistent(persistent bool) *logPipelineV1Alpha1 {
	p.persistent = persistent

	return p
}

func (p *logPipelineV1Alpha1) K8sObject() *telemetryv1alpha1.LogPipeline {
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
