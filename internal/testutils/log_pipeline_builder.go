package testutils

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogPipelineBuilder struct {
	randSource rand.Source

	name              string
	labels            map[string]string
	deletionTimeStamp metav1.Time

	includeContainers []string
	excludeContainers []string
	includeNamespaces []string
	excludeNamespaces []string
	systemNamespaces  bool
	keepAnnotations   bool
	dropLabels        bool

	customFilter string

	httpOutput   *telemetryv1alpha1.HTTPOutput
	lokiOutput   *telemetryv1alpha1.LokiOutput
	customOutput string

	statusConditions []metav1.Condition
}

func NewLogPipelineBuilder() *LogPipelineBuilder {
	return &LogPipelineBuilder{
		randSource: rand.NewSource(time.Now().UnixNano()),
		labels:     make(map[string]string),
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

func (b *LogPipelineBuilder) WithIncludeContainers(containers ...string) *LogPipelineBuilder {
	b.includeContainers = containers
	return b
}

func (b *LogPipelineBuilder) WithExcludeContainers(containers ...string) *LogPipelineBuilder {
	b.excludeContainers = containers
	return b
}

func (b *LogPipelineBuilder) WithIncludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	b.includeNamespaces = namespaces
	return b
}

func (b *LogPipelineBuilder) WithExcludeNamespaces(namespaces ...string) *LogPipelineBuilder {
	b.excludeNamespaces = namespaces
	return b
}

func (b *LogPipelineBuilder) WithSystemNamespaces(enable bool) *LogPipelineBuilder {
	b.systemNamespaces = enable
	return b
}

func (b *LogPipelineBuilder) WithKeepAnnotations(keep bool) *LogPipelineBuilder {
	b.keepAnnotations = keep
	return b
}

func (b *LogPipelineBuilder) WithDropLabels(drop bool) *LogPipelineBuilder {
	b.dropLabels = drop
	return b
}

func (b *LogPipelineBuilder) WithCustomFilter(filter string) *LogPipelineBuilder {
	b.customFilter = filter
	return b
}

func (b *LogPipelineBuilder) WithHTTPOutput(opts ...HTTPOutputOption) *LogPipelineBuilder {
	for _, opt := range opts {
		opt(b.httpOutput)
	}
	return b
}

func (b *LogPipelineBuilder) WithLokiOutput() *LogPipelineBuilder {
	b.lokiOutput = &telemetryv1alpha1.LokiOutput{
		URL: telemetryv1alpha1.ValueType{Value: "https://localhost:3100"},
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

func (b *LogPipelineBuilder) Build() telemetryv1alpha1.LogPipeline {
	if b.name == "" {
		b.name = fmt.Sprintf("test-%d", b.randSource.Int63())
	}
	if b.httpOutput == nil || b.lokiOutput == nil || b.customOutput == "" {
		b.httpOutput = &telemetryv1alpha1.HTTPOutput{
			Host: telemetryv1alpha1.ValueType{Value: "https://localhost:4317"},
		}
	}

	logPipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:   b.name,
			Labels: b.labels,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Containers: telemetryv1alpha1.InputContainers{
						Include: b.includeContainers,
						Exclude: b.excludeContainers,
					},
					Namespaces: telemetryv1alpha1.InputNamespaces{
						Include: b.includeNamespaces,
						Exclude: b.excludeNamespaces,
						System:  b.systemNamespaces,
					},
					KeepAnnotations: b.keepAnnotations,
					DropLabels:      b.dropLabels,
				},
			},
			Output: telemetryv1alpha1.Output{
				HTTP:   b.httpOutput,
				Loki:   b.lokiOutput,
				Custom: b.customOutput,
			},
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
