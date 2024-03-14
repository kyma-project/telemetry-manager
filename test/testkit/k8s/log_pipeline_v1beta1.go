package k8s

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend/tls"
)

//nolint:dupl //There is duplication between logPipelineV1Beta1 and logPipelineV1Alpha1, but we need them as separate builders because they are using different API versions
type logPipelinev1beta1 struct {
	persistent bool

	name              string
	secretKeyRef      *telemetryv1beta1.SecretKeyRef
	systemNamespaces  bool
	includeNamespaces []string
	excludeNamespaces []string
	includeContainers []string
	excludeContainers []string
	keepAnnotations   bool
	dropLabels        bool
	output            telemetryv1beta1.Output
	filters           []telemetryv1beta1.Filter
}

func NewLogPipelinev1beta1(name string) *logPipelinev1beta1 {
	return &logPipelinev1beta1{
		name: name,
	}
}

func (p *logPipelinev1beta1) Name() string {
	return p.name
}

func (p *logPipelinev1beta1) WithSecretKeyRef(secretKeyRef *telemetryv1beta1.SecretKeyRef) *logPipelinev1beta1 {
	p.secretKeyRef = secretKeyRef
	return p
}

func (p *logPipelinev1beta1) WithSystemNamespaces(enable bool) *logPipelinev1beta1 {
	p.systemNamespaces = enable
	return p
}

func (p *logPipelinev1beta1) WithIncludeNamespaces(namespaces []string) *logPipelinev1beta1 {
	p.includeNamespaces = namespaces
	return p
}

func (p *logPipelinev1beta1) WithExcludeNamespaces(namespaces []string) *logPipelinev1beta1 {
	p.excludeNamespaces = namespaces
	return p
}

func (p *logPipelinev1beta1) WithIncludeContainers(names []string) *logPipelinev1beta1 {
	p.includeContainers = names
	return p
}

func (p *logPipelinev1beta1) WithExcludeContainers(names []string) *logPipelinev1beta1 {
	p.excludeContainers = names
	return p
}

func (p *logPipelinev1beta1) KeepAnnotations(enable bool) *logPipelinev1beta1 {
	p.keepAnnotations = enable
	return p
}

func (p *logPipelinev1beta1) DropLabels(enable bool) *logPipelinev1beta1 {
	p.dropLabels = enable
	return p
}

func (p *logPipelinev1beta1) WithStdout() *logPipelinev1beta1 {
	p.output = telemetryv1beta1.Output{
		Custom: "Name stdout",
	}
	return p
}

func (p *logPipelinev1beta1) WithHTTPOutput() *logPipelinev1beta1 {
	p.output = telemetryv1beta1.Output{
		HTTP: &telemetryv1beta1.HTTPOutput{
			Dedot: true,
			Host: telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: p.secretKeyRef,
				},
			},
			Port:   "9880",
			URI:    "/",
			Format: "json",
			TLSConfig: telemetryv1beta1.TLSConfig{
				Disabled:                  true,
				SkipCertificateValidation: true,
			},
		},
	}
	return p
}

func (p *logPipelinev1beta1) WithTLS(certs tls.Certs) *logPipelinev1beta1 {
	if !p.output.IsHTTPDefined() {
		return p
	}

	p.output.HTTP.TLSConfig = telemetryv1beta1.TLSConfig{
		Disabled:                  false,
		SkipCertificateValidation: false,
		CA: &telemetryv1beta1.ValueType{
			Value: certs.CaCertPem.String(),
		},
		Cert: &telemetryv1beta1.ValueType{
			Value: certs.ClientCertPem.String(),
		},
		Key: &telemetryv1beta1.ValueType{
			Value: certs.ClientKeyPem.String(),
		},
	}

	return p
}

func (p *logPipelinev1beta1) WithCustomOutput(host string) *logPipelinev1beta1 {
	const customOutputTemplate = `
	name   http
	port   9880
	host   {{ HOST }}
	format json`
	customOutput := strings.Replace(customOutputTemplate, "{{ HOST }}", host, 1)
	p.output = telemetryv1beta1.Output{
		Custom: customOutput,
	}
	return p
}

func (p *logPipelinev1beta1) WithFilter(filter string) *logPipelinev1beta1 {
	p.filters = append(p.filters, telemetryv1beta1.Filter{
		Custom: filter,
	})
	return p
}

func (p *logPipelinev1beta1) Persistent(persistent bool) *logPipelinev1beta1 {
	p.persistent = persistent

	return p
}

func (p *logPipelinev1beta1) K8sObject() *telemetryv1beta1.LogPipeline {
	var labels Labels
	if p.persistent {
		labels = PersistentLabel
	}

	return &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.name,
			Labels: labels,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.Input{
				Application: telemetryv1beta1.ApplicationInput{
					Namespaces: telemetryv1beta1.InputNamespaces{
						System:  p.systemNamespaces,
						Include: p.includeNamespaces,
						Exclude: p.excludeNamespaces,
					},
					Containers: telemetryv1beta1.InputContainers{
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
