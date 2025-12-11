package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

const name = "name"

func newMetricPipelineAlpha() *telemetryv1alpha1.MetricPipeline {
	return &telemetryv1alpha1.MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newTracePipelineAlpha() *telemetryv1alpha1.TracePipeline {
	return &telemetryv1alpha1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newLogPipelineAlpha() *telemetryv1alpha1.LogPipeline {
	return &telemetryv1alpha1.LogPipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newMetricPipelineBeta() *telemetryv1beta1.MetricPipeline {
	return &telemetryv1beta1.MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newTracePipelineBeta() *telemetryv1beta1.TracePipeline {
	return &telemetryv1beta1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newLogPipelineBeta() *telemetryv1beta1.LogPipeline {
	return &telemetryv1beta1.LogPipeline{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func Test_sortObjects(t *testing.T) {
	tests := []struct {
		name          string
		input         []client.Object
		wantMixed     []client.Object
		wantPipelines []client.Object
	}{
		{
			name: "namespaces first, pipelines last, mixed types",
			input: []client.Object{
				newMetricPipelineAlpha(),
				newConfigMap("cfg"),
				newNamespace("ns"),
				newMetricPipelineBeta(),
				newTracePipelineAlpha(),
				newLogPipelineBeta(),
			},
			wantMixed: []client.Object{
				newNamespace("ns"),
				newConfigMap("cfg"),
			},
			wantPipelines: []client.Object{
				newMetricPipelineAlpha(),
				newMetricPipelineBeta(),
				newTracePipelineAlpha(),
				newLogPipelineBeta(),
			},
		},
		{
			name: "only namespaces",
			input: []client.Object{
				newNamespace("ns1"),
				newNamespace("ns2"),
			},
			wantMixed: []client.Object{
				newNamespace("ns1"),
				newNamespace("ns2"),
			},
		},
		{
			name: "only pipelines",
			input: []client.Object{
				newMetricPipelineAlpha(),
				newLogPipelineAlpha(),
				newTracePipelineBeta(),
			},
			wantPipelines: []client.Object{
				newMetricPipelineAlpha(),
				newLogPipelineAlpha(),
				newTracePipelineBeta(),
			},
		},
		{
			name: "only other types",
			input: []client.Object{
				newConfigMap("cfg1"),
				newConfigMap("cfg2"),
			},
			wantMixed: []client.Object{
				newConfigMap("cfg1"),
				newConfigMap("cfg2"),
			},
		},
		{
			name:  "empty input returns empty",
			input: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mixed, pipelines := sortObjects(tt.input)
			if !reflect.DeepEqual(mixed, tt.wantMixed) {
				t.Errorf("sortObjects() = %v, want %v", mixed, tt.wantMixed)
			}

			if !reflect.DeepEqual(pipelines, tt.wantPipelines) {
				t.Errorf("sortObjects() = %v, want %v", pipelines, tt.wantPipelines)
			}
		})
	}
}
