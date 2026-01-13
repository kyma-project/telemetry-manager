package test

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name              string
	labels            map[string]string
	finalizers        []string
	deletionTimeStamp metav1.Time

	input telemetryv1beta1.LogPipelineInput

	httpOutput   *telemetryv1beta1.FluentBitHTTPOutput
	otlpOutput   *telemetryv1beta1.OTLPOutput
	oauth2       *telemetryv1beta1.OAuth2Options
	customOutput string

	fluentBitFilters []telemetryv1beta1.FluentBitFilter
	files            []telemetryv1beta1.FluentBitFile
	variables        []telemetryv1beta1.FluentBitVariable
	transforms       []telemetryv1beta1.TransformSpec
	filters          []telemetryv1beta1.FilterSpec

	statusConditions []metav1.Condition
}

func BuildLogPipelineRuntimeInput(opts ...NamespaceSelectorOptions) telemetryv1beta1.LogPipelineInput {
	input := telemetryv1beta1.LogPipelineInput{
		Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
			Enabled: ptr.To(true),
		},
	}

	if len(opts) == 0 {
		return input
	}

	input.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{}

	for _, opt := range opts {
		opt(input.Runtime.Namespaces)
	}

	return input
}

func BuildLogPipelineOTLPInput(opts ...NamespaceSelectorOptions) telemetryv1beta1.LogPipelineInput {
	input := telemetryv1beta1.LogPipelineInput{
		Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
			Enabled: ptr.To(false),
		},
		OTLP: &telemetryv1beta1.OTLPInput{
			Enabled:    ptr.To(true),
			Namespaces: &telemetryv1beta1.NamespaceSelector{},
		},
	}

	if len(opts) == 0 {
		return input
	}

	for _, opt := range opts {
		opt(input.OTLP.Namespaces)
	}

	return input
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

func (b *LogPipelineBuilder) WithInput(input telemetryv1beta1.LogPipelineInput) *LogPipelineBuilder {
	b.input = input
	return b
}

func (b *LogPipelineBuilder) WithOutput(output telemetryv1beta1.LogPipelineOutput) *LogPipelineBuilder {
	b.httpOutput = output.FluentBitHTTP
	b.customOutput = output.FluentBitCustom
	b.otlpOutput = output.OTLP

	return b
}

func (b *LogPipelineBuilder) WithRuntimeInput(enabled bool, opts ...NamespaceSelectorOptions) *LogPipelineBuilder {
	b.input = BuildLogPipelineRuntimeInput(opts...)
	b.input.Runtime.Enabled = ptr.To(enabled)
	b.input.Runtime.KeepOriginalBody = ptr.To(false)

	return b
}

func (b *LogPipelineBuilder) WithOTLPInput(enabled bool, opts ...NamespaceSelectorOptions) *LogPipelineBuilder {
	b.input = BuildLogPipelineOTLPInput(opts...)
	b.input.OTLP.Enabled = ptr.To(enabled)

	return b
}

func (b *LogPipelineBuilder) WithIncludeContainers(containers ...string) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	if b.input.Runtime.Containers == nil {
		b.input.Runtime.Containers = &telemetryv1beta1.LogPipelineContainerSelector{}
	}

	b.input.Runtime.Containers.Include = containers

	return b
}

func (b *LogPipelineBuilder) WithExcludeContainers(containers ...string) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	if b.input.Runtime.Containers == nil {
		b.input.Runtime.Containers = &telemetryv1beta1.LogPipelineContainerSelector{}
	}

	b.input.Runtime.Containers.Exclude = containers

	return b
}

func (b *LogPipelineBuilder) WithIncludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	if b.input.Runtime.Namespaces == nil {
		b.input.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	b.input.Runtime.Namespaces.Include = namespaces

	return b
}

func (b *LogPipelineBuilder) WithExcludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	if b.input.Runtime.Namespaces == nil {
		b.input.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{}
	}

	b.input.Runtime.Namespaces.Exclude = namespaces

	return b
}

func (b *LogPipelineBuilder) WithKeepAnnotations(keep bool) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	b.input.Runtime.FluentBitKeepAnnotations = &keep

	return b
}

func (b *LogPipelineBuilder) WithDropLabels(drop bool) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	b.input.Runtime.FluentBitDropLabels = &drop

	return b
}

func (b *LogPipelineBuilder) WithKeepOriginalBody(keep bool) *LogPipelineBuilder {
	if b.input.Runtime == nil {
		b.input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{}
	}

	b.input.Runtime.KeepOriginalBody = ptr.To(keep)

	return b
}

func (b *LogPipelineBuilder) WithCustomFilter(filter string) *LogPipelineBuilder {
	b.fluentBitFilters = append(b.fluentBitFilters, telemetryv1beta1.FluentBitFilter{Custom: filter})
	return b
}

func (b *LogPipelineBuilder) WithFile(name, content string) *LogPipelineBuilder {
	b.files = append(b.files, telemetryv1beta1.FluentBitFile{
		Name:    name,
		Content: content,
	})

	return b
}

func (b *LogPipelineBuilder) WithVariable(name, secretName, secretNamespace, secretKey string) *LogPipelineBuilder {
	b.variables = append(b.variables, telemetryv1beta1.FluentBitVariable{
		Name: name,
		ValueFrom: telemetryv1beta1.ValueFromSource{
			SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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

func (b *LogPipelineBuilder) WithOAuth2(opts ...OAuth2Option) *LogPipelineBuilder {
	if b.oauth2 == nil {
		b.oauth2 = &telemetryv1beta1.OAuth2Options{}
	}

	for _, opt := range opts {
		opt(b.oauth2)
	}

	// Set OAuth2 on the OTLP output authentication
	if b.otlpOutput == nil {
		b.otlpOutput = defaultOTLPOutput()
	}

	if b.otlpOutput.Authentication == nil {
		b.otlpOutput.Authentication = &telemetryv1beta1.AuthenticationOptions{}
	}

	b.otlpOutput.Authentication.OAuth2 = b.oauth2

	return b
}

func (b *LogPipelineBuilder) WithCustomOutput(custom string) *LogPipelineBuilder {
	b.customOutput = custom
	return b
}

func (b *LogPipelineBuilder) WithTransform(transform telemetryv1beta1.TransformSpec) *LogPipelineBuilder {
	b.transforms = append(b.transforms, transform)
	return b
}

func (b *LogPipelineBuilder) WithFilter(filter telemetryv1beta1.FilterSpec) *LogPipelineBuilder {
	b.filters = append(b.filters, filter)
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

func (b *LogPipelineBuilder) Build() telemetryv1beta1.LogPipeline {
	if b.name == "" {
		b.name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}

	if b.httpOutput == nil && b.customOutput == "" && b.otlpOutput == nil {
		b.httpOutput = defaultHTTPOutput()
	}

	logPipeline := telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:       b.name,
			Labels:     b.labels,
			Finalizers: b.finalizers,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input:            b.input,
			FluentBitFilters: b.fluentBitFilters,
			Output: telemetryv1beta1.LogPipelineOutput{
				FluentBitHTTP:   b.httpOutput,
				FluentBitCustom: b.customOutput,
				OTLP:            b.otlpOutput,
			},
			FluentBitFiles:     b.files,
			FluentBitVariables: b.variables,
			Transforms:         b.transforms,
			Filters:            b.filters,
		},
		Status: telemetryv1beta1.LogPipelineStatus{
			Conditions: b.statusConditions,
		},
	}
	if !b.deletionTimeStamp.IsZero() {
		logPipeline.DeletionTimestamp = &b.deletionTimeStamp
	}

	return logPipeline
}

func defaultHTTPOutput() *telemetryv1beta1.FluentBitHTTPOutput {
	return &telemetryv1beta1.FluentBitHTTPOutput{
		Host:   telemetryv1beta1.ValueType{Value: "127.0.0.1"},
		Port:   "8080",
		URI:    "/",
		Format: "json",
		TLS: telemetryv1beta1.OutputTLS{
			Insecure:           true,
			InsecureSkipVerify: true,
		},
	}
}

func defaultOTLPOutput() *telemetryv1beta1.OTLPOutput {
	return &telemetryv1beta1.OTLPOutput{
		Endpoint: telemetryv1beta1.ValueType{Value: "https://localhost:4317"},
		Protocol: telemetryv1beta1.OTLPProtocolGRPC,
	}
}
