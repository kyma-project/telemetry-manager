package test

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name              string
	labels            map[string]string
	finalizers        []string
	deletionTimeStamp metav1.Time

	input            telemetryv1alpha1.LogPipelineInput
	filters          []telemetryv1alpha1.LogPipelineFilter
	httpOutput       *telemetryv1alpha1.LogPipelineHTTPOutput
	otlpOutput       *telemetryv1alpha1.OTLPOutput
	customOutput     string
	files            []telemetryv1alpha1.LogPipelineFileMount
	variables        []telemetryv1alpha1.LogPipelineVariableRef
	statusConditions []metav1.Condition
}

func NewLogPipelineBuilder() *LogPipelineBuilder {
	return &LogPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
	}
}

func (b *LogPipelineBuilder) WithName(name string) *LogPipelineBuilder {
	b.name = name
	return b
}

func (b *LogPipelineBuilder) WithLabels(labels map[string]string) *LogPipelineBuilder {
	b.labels = labels
	return b
}

func (b *LogPipelineBuilder) WithFinalizer(finalizer string) *LogPipelineBuilder {
	b.finalizers = append(b.finalizers, finalizer)
	return b
}

func (b *LogPipelineBuilder) WithApplicationInputDisabled() *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Enabled = ptr.To(false)

	return b
}

func (b *LogPipelineBuilder) WithApplicationInput(enabled bool) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}
	b.input.Application.Enabled = ptr.To(enabled)

	return b
}

func (b *LogPipelineBuilder) WithOTLPInput() *LogPipelineBuilder {
	if b.input.OTLP == nil {
		b.input.OTLP = &telemetryv1alpha1.OTLPInput{}
	}

	return b
}

func (b *LogPipelineBuilder) WithIncludeContainers(containers ...string) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Containers.Include = containers

	return b
}

func (b *LogPipelineBuilder) WithExcludeContainers(containers ...string) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Containers.Exclude = containers

	return b
}

func (b *LogPipelineBuilder) WithIncludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Namespaces.Include = namespaces

	return b
}

func (b *LogPipelineBuilder) WithExcludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Namespaces.Exclude = namespaces

	return b
}

func (b *LogPipelineBuilder) WithSystemNamespaces(enable bool) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.Namespaces.System = enable

	return b
}

func (b *LogPipelineBuilder) WithKeepAnnotations(keep bool) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.KeepAnnotations = keep

	return b
}

func (b *LogPipelineBuilder) WithDropLabels(drop bool) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.DropLabels = drop

	return b
}

func (b *LogPipelineBuilder) WithKeepOriginalBody(keep bool) *LogPipelineBuilder {
	if b.input.Application == nil {
		b.input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{}
	}

	b.input.Application.KeepOriginalBody = ptr.To(keep)

	return b
}

func (b *LogPipelineBuilder) WithCustomFilter(filter string) *LogPipelineBuilder {
	b.filters = append(b.filters, telemetryv1alpha1.LogPipelineFilter{Custom: filter})
	return b
}

func (b *LogPipelineBuilder) WithFile(name, content string) *LogPipelineBuilder {
	b.files = append(b.files, telemetryv1alpha1.LogPipelineFileMount{
		Name:    name,
		Content: content,
	})

	return b
}

func (b *LogPipelineBuilder) WithVariable(name, secretName, secretNamespace, secretKey string) *LogPipelineBuilder {
	b.variables = append(b.variables, telemetryv1alpha1.LogPipelineVariableRef{
		Name: name,
		ValueFrom: telemetryv1alpha1.ValueFromSource{
			SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
				Name:      secretName,
				Namespace: secretNamespace,
				Key:       secretKey,
			},
		},
	})

	return b
}

func (b *LogPipelineBuilder) WithHTTPOutput(opts ...HTTPOutputOption) *LogPipelineBuilder {
	b.httpOutput = defaultHTTPOutput()
	for _, opt := range opts {
		opt(b.httpOutput)
	}

	return b
}

func (b *LogPipelineBuilder) WithOTLPOutput(opts ...OTLPOutputOption) *LogPipelineBuilder {
	b.otlpOutput = defaultOTLPOutput()
	for _, opt := range opts {
		opt(b.otlpOutput)
	}

	return b
}

func (b *LogPipelineBuilder) WithCustomOutput(custom string) *LogPipelineBuilder {
	b.customOutput = custom
	return b
}

func (b *LogPipelineBuilder) WithDeletionTimeStamp(ts metav1.Time) *LogPipelineBuilder {
	b.deletionTimeStamp = ts
	return b
}

func (b *LogPipelineBuilder) WithStatusCondition(cond metav1.Condition) *LogPipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond)
	return b
}

func (b *LogPipelineBuilder) WithStatusConditions(cond ...metav1.Condition) *LogPipelineBuilder {
	b.statusConditions = append(b.statusConditions, cond...)
	return b
}

func (b *LogPipelineBuilder) Build() telemetryv1alpha1.LogPipeline {
	if b.name == "" {
		b.name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	if b.httpOutput == nil && b.customOutput == "" && b.otlpOutput == nil {
		b.httpOutput = defaultHTTPOutput()
	}

	logPipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       b.name,
			Labels:     b.labels,
			Finalizers: b.finalizers,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input:   b.input,
			Filters: b.filters,
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP:   b.httpOutput,
				Custom: b.customOutput,
				OTLP:   b.otlpOutput,
			},
			Files:     b.files,
			Variables: b.variables,
		},
		Status: telemetryv1alpha1.LogPipelineStatus{
			Conditions: b.statusConditions,
		},
	}
	if !b.deletionTimeStamp.IsZero() {
		logPipeline.DeletionTimestamp = &b.deletionTimeStamp
	}

	return logPipeline
}

func defaultHTTPOutput() *telemetryv1alpha1.LogPipelineHTTPOutput {
	return &telemetryv1alpha1.LogPipelineHTTPOutput{
		Host:   telemetryv1alpha1.ValueType{Value: "127.0.0.1"},
		Port:   "8080",
		URI:    "/",
		Format: "json",
		TLS: telemetryv1alpha1.LogPipelineOutputTLS{
			Disabled:                  true,
			SkipCertificateValidation: true,
		},
	}
}

func defaultOTLPOutput() *telemetryv1alpha1.OTLPOutput {
	return &telemetryv1alpha1.OTLPOutput{
		Endpoint: telemetryv1alpha1.ValueType{Value: "https://localhost:4317"},
		Protocol: telemetryv1alpha1.OTLPProtocolGRPC,
	}
}
