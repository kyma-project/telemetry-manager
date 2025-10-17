package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestMetricPipelineConvertTo(t *testing.T) {
	tests := []struct {
		name     string
		input    *MetricPipeline
		expected *telemetryv1beta1.MetricPipeline
	}{
		{
			name: "should convert basic fields",
			input: &MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-ns",
				},
				Spec: MetricPipelineSpec{},
			},
			expected: &telemetryv1beta1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-ns",
				},
				Spec: telemetryv1beta1.MetricPipelineSpec{},
			},
		},
		{
			name: "should sanitize namespace selectors",
			input: &MetricPipeline{
				Spec: MetricPipelineSpec{
					Input: MetricPipelineInput{
						Runtime: &MetricPipelineRuntimeInput{
							Namespaces: &NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Input: telemetryv1beta1.MetricPipelineInput{
						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"valid-ns", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "another-valid-excluded"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &telemetryv1beta1.MetricPipeline{}
			err := tt.input.ConvertTo(dst)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dst)
		})
	}
}

func TestMetricPipelineConvertFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1beta1.MetricPipeline
		expected *MetricPipeline
	}{
		{
			name: "should convert basic fields",
			input: &telemetryv1beta1.MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-ns",
				},
				Spec: telemetryv1beta1.MetricPipelineSpec{},
			},
			expected: &MetricPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-ns",
				},
				Spec: MetricPipelineSpec{},
			},
		},
		{
			name: "should convert namespace selectors without validation",
			input: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Input: telemetryv1beta1.MetricPipelineInput{
						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS"},
								Exclude: []string{"valid-excluded", "Invalid@NS"},
							},
						},
					},
				},
			},
			expected: &MetricPipeline{
				Spec: MetricPipelineSpec{
					Input: MetricPipelineInput{
						Runtime: &MetricPipelineRuntimeInput{
							Namespaces: &NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS"},
								Exclude: []string{"valid-excluded", "Invalid@NS"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &MetricPipeline{}
			err := dst.ConvertFrom(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dst)
		})
	}
}

func TestMetricPipelineConvertTo_InvalidDestination(t *testing.T) {
	src := &MetricPipeline{}
	dst := &MetricPipeline{} // Wrong type for destination
	err := src.ConvertTo(dst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dst object must be of type *v1beta1.MetricPipeline")
}

func TestMetricPipelineConvertFrom_InvalidSource(t *testing.T) {
	src := &MetricPipeline{} // Wrong type for source
	dst := &MetricPipeline{}
	err := dst.ConvertFrom(src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "src object must be of type *v1beta1.MetricPipeline")
}

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func boolPtr(b bool) *bool {

	return &b

}	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"



func TestMetricPipelineConvertTo(t *testing.T) {	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tests := []struct {

		name     string))

		input    *MetricPipeline

		expected *telemetryv1beta1.MetricPipeline

	}{

		{func boolPtr(b bool) *bool {func boolPtr(b bool) *bool {

			name: "should convert basic fields",

			input: &MetricPipeline{	return &b	return &b

				ObjectMeta: metav1.ObjectMeta{

					Name:      "test-pipeline",}}

					Namespace: "test-ns",

				},

				Spec: MetricPipelineSpec{},

			},func TestMetricPipelineConvertTo(t *testing.T) {func TestMetricPipelineConvertTo(t *testing.T) {

			expected: &telemetryv1beta1.MetricPipeline{

				ObjectMeta: metav1.ObjectMeta{	tests := []struct {	tests := []struct {

					Name:      "test-pipeline",

					Namespace: "test-ns",		name     string		name     string

				},

				Spec: telemetryv1beta1.MetricPipelineSpec{},		input    *MetricPipeline		input    *MetricPipeline

			},

		},		expected *telemetryv1beta1.MetricPipeline		expected *telemetryv1beta1.MetricPipeline

		{

			name: "should sanitize namespaces in namespace selectors",	}{	}{

			input: &MetricPipeline{

				ObjectMeta: metav1.ObjectMeta{		{		{

					Name: "test-pipeline",

				},			name: "should convert basic fields",			name: "should convert basic fields",

				Spec: MetricPipelineSpec{

					Input: MetricPipelineInput{			input: &MetricPipeline{			input: &MetricPipeline{

						Runtime: &MetricPipelineRuntimeInput{

							Namespaces: &NamespaceSelector{				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},

								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},					Name:      "test-pipeline",					Name:      "test-pipeline",

							},

						},					Namespace: "test-ns",					Namespace: "test-ns",

					},

				},				},				},

			},

			expected: &telemetryv1beta1.MetricPipeline{				Spec: MetricPipelineSpec{},				Spec: MetricPipelineSpec{},

				ObjectMeta: metav1.ObjectMeta{

					Name: "test-pipeline",			},			},

				},

				Spec: telemetryv1beta1.MetricPipelineSpec{			expected: &telemetryv1beta1.MetricPipeline{			expected: &telemetryv1beta1.MetricPipeline{

					Input: telemetryv1beta1.MetricPipelineInput{

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

							Namespaces: &telemetryv1beta1.NamespaceSelector{

								Include: []string{"valid-ns", "another-valid-ns"},					Name:      "test-pipeline",					Name:      "test-pipeline",

								Exclude: []string{"valid-excluded", "another-valid-excluded"},

							},					Namespace: "test-ns",					Namespace: "test-ns",

						},

					},				},				},

				},

			},				Spec: telemetryv1beta1.MetricPipelineSpec{},				Spec: telemetryv1beta1.MetricPipelineSpec{},

		},

		{			},			},

			name: "should convert all input sections",

			input: &MetricPipeline{		},		},

				Spec: MetricPipelineSpec{

					Input: MetricPipelineInput{		{		{

						Prometheus: &MetricPipelinePrometheusInput{

							Enabled: boolPtr(true),			name: "should sanitize namespaces in namespace selectors",			name: "should sanitize namespaces in namespace selectors",

							Namespaces: &NamespaceSelector{

								Include: []string{"ns1"},			input: &MetricPipeline{			input: &MetricPipeline{

							},

						},				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

						Runtime: &MetricPipelineRuntimeInput{

							Enabled: boolPtr(true),					Name: "test-pipeline",					Name: "test-pipeline",

							Resources: &MetricPipelineRuntimeInputResources{

								Pod: &MetricPipelineRuntimeInputResource{				},				},

									Enabled: boolPtr(true),

								},				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

							},

						},					Input: MetricPipelineInput{					Input: MetricPipelineInput{

						Istio: &MetricPipelineIstioInput{

							Enabled: boolPtr(true),						Runtime: &MetricPipelineRuntimeInput{						Runtime: &MetricPipelineRuntimeInput{

							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{

								Enabled: boolPtr(true),							Namespaces: &NamespaceSelector{							Namespaces: &NamespaceSelector{

							},

						},								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},

						OTLP: &OTLPInput{

							Disabled: boolPtr(true),								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},

						},

					},							},							},

				},

			},						},						},

			expected: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{					},					},

					Input: telemetryv1beta1.MetricPipelineInput{

						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{				},				},

							Enabled: boolPtr(true),

							Namespaces: &telemetryv1beta1.NamespaceSelector{			},			},

								Include: []string{"ns1"},

							},			expected: &telemetryv1beta1.MetricPipeline{			expected: &telemetryv1beta1.MetricPipeline{

						},

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

							Enabled: boolPtr(true),

							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{					Name: "test-pipeline",					Name: "test-pipeline",

								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{

									Enabled: boolPtr(true),				},				},

								},

							},				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

						},

						Istio: &telemetryv1beta1.MetricPipelineIstioInput{					Input: telemetryv1beta1.MetricPipelineInput{					Input: telemetryv1beta1.MetricPipelineInput{

							Enabled: boolPtr(true),

							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{

								Enabled: boolPtr(true),

							},							Namespaces: &telemetryv1beta1.NamespaceSelector{							Namespaces: &telemetryv1beta1.NamespaceSelector{

						},

						OTLP: &telemetryv1beta1.OTLPInput{								Include: []string{"valid-ns", "another-valid-ns"},								Include: []string{"valid-ns", "another-valid-ns"},

							Disabled: boolPtr(true),

						},								Exclude: []string{"valid-excluded", "another-valid-excluded"},								Exclude: []string{"valid-excluded", "another-valid-excluded"},

					},

				},							},							},

			},

		},						},						},

		{

			name: "should convert output section with authentication",					},					},

			input: &MetricPipeline{

				Spec: MetricPipelineSpec{				},				},

					Output: MetricPipelineOutput{

						OTLP: &OTLPOutput{			},			},

							Protocol: "grpc",

							Endpoint: ValueType{		},		},

								Value: "localhost:4317",

							},		{		{

							Authentication: &AuthenticationOptions{

								Basic: &BasicAuthOptions{			name: "should convert all input sections",			name: "should convert all input sections",

									User: ValueType{

										Value: "user",			input: &MetricPipeline{			input: &MetricPipeline{

									},

									Password: ValueType{				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

										Value: "pass",

									},					Input: MetricPipelineInput{					Input: MetricPipelineInput{

								},

							},						Prometheus: &MetricPipelinePrometheusInput{						Prometheus: &MetricPipelinePrometheusInput{

						},

					},							Enabled: boolPtr(true),							Enabled: true,

				},

			},							Namespaces: &NamespaceSelector{							Namespaces: &NamespaceSelector{

			expected: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{								Include: []string{"ns1"},								Include: []string{"ns1"},

					Output: telemetryv1beta1.MetricPipelineOutput{

						OTLP: &telemetryv1beta1.OTLPOutput{							},							},

							Protocol: "grpc",

							Endpoint: telemetryv1beta1.ValueType{						},						},

								Value: "localhost:4317",

							},						Runtime: &MetricPipelineRuntimeInput{						Runtime: &MetricPipelineRuntimeInput{

							Authentication: &telemetryv1beta1.AuthenticationOptions{

								Basic: &telemetryv1beta1.BasicAuthOptions{							Enabled: boolPtr(true),							Enabled: true,

									User: telemetryv1beta1.ValueType{

										Value: "user",							Resources: &MetricPipelineRuntimeInputResources{							Resources: &MetricPipelineRuntimeInputResources{

									},

									Password: telemetryv1beta1.ValueType{								Pod: &MetricPipelineRuntimeInputResource{								Pod: &MetricPipelineRuntimeInputResource{

										Value: "pass",

									},									Enabled: boolPtr(true),									Enabled: true,

								},

							},								},								},

						},

					},							},							},

				},

			},						},						},

		},

	}						Istio: &MetricPipelineIstioInput{						Istio: &MetricPipelineIstioInput{



	for _, tt := range tests {							Enabled: boolPtr(true),							Enabled: true,

		t.Run(tt.name, func(t *testing.T) {

			dst := &telemetryv1beta1.MetricPipeline{}							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{

			err := tt.input.ConvertTo(dst)

			assert.NoError(t, err)								Enabled: boolPtr(true),								Enabled: true,

			assert.Equal(t, tt.expected, dst)

		})							},							},

	}

}						},						},



func TestMetricPipelineConvertFrom(t *testing.T) {						OTLP: &OTLPInput{						OTLP: &OTLPInput{

	tests := []struct {

		name     string							Disabled: boolPtr(true),							Disabled: true,

		input    *telemetryv1beta1.MetricPipeline

		expected *MetricPipeline						},						},

	}{

		{					},					},

			name: "should convert basic fields",

			input: &telemetryv1beta1.MetricPipeline{				},				},

				ObjectMeta: metav1.ObjectMeta{

					Name:      "test-pipeline",			},			},

					Namespace: "test-ns",

				},			expected: &telemetryv1beta1.MetricPipeline{			expected: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{},

			},				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

			expected: &MetricPipeline{

				ObjectMeta: metav1.ObjectMeta{					Input: telemetryv1beta1.MetricPipelineInput{					Input: telemetryv1beta1.MetricPipelineInput{

					Name:      "test-pipeline",

					Namespace: "test-ns",						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{

				},

				Spec: MetricPipelineSpec{},							Enabled: boolPtr(true),							Enabled: true,

			},

		},							Namespaces: &telemetryv1beta1.NamespaceSelector{							Namespaces: &telemetryv1beta1.NamespaceSelector{

		{

			name: "should convert namespaces in namespace selectors without validation",								Include: []string{"ns1"},								Include: []string{"ns1"},

			input: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{							},							},

					Input: telemetryv1beta1.MetricPipelineInput{

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{						},						},

							Namespaces: &telemetryv1beta1.NamespaceSelector{

								Include: []string{"valid-ns", "Invalid_NS"},						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{

								Exclude: []string{"valid-excluded", "Invalid@NS"},

							},							Enabled: boolPtr(true),							Enabled: true,

						},

					},							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{

				},

			},								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{

			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{									Enabled: boolPtr(true),									Enabled: true,

					Input: MetricPipelineInput{

						Runtime: &MetricPipelineRuntimeInput{								},								},

							Namespaces: &NamespaceSelector{

								Include: []string{"valid-ns", "Invalid_NS"},							},							},

								Exclude: []string{"valid-excluded", "Invalid@NS"},

							},						},						},

						},

					},						Istio: &telemetryv1beta1.MetricPipelineIstioInput{						Istio: &telemetryv1beta1.MetricPipelineIstioInput{

				},

			},							Enabled: boolPtr(true),							Enabled: true,

		},

		{							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{

			name: "should convert all input sections",

			input: &telemetryv1beta1.MetricPipeline{								Enabled: boolPtr(true),								Enabled: true,

				Spec: telemetryv1beta1.MetricPipelineSpec{

					Input: telemetryv1beta1.MetricPipelineInput{							},							},

						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{

							Enabled: boolPtr(true),						},						},

							Namespaces: &telemetryv1beta1.NamespaceSelector{

								Include: []string{"ns1"},						OTLP: &telemetryv1beta1.OTLPInput{						OTLP: &telemetryv1beta1.OTLPInput{

							},

						},							Disabled: boolPtr(true),							Disabled: true,

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{

							Enabled: boolPtr(true),						},						},

							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{

								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{					},					},

									Enabled: boolPtr(true),

								},				},				},

							},

						},			},			},

						Istio: &telemetryv1beta1.MetricPipelineIstioInput{

							Enabled: boolPtr(true),		},		},

							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{

								Enabled: boolPtr(true),		{		{

							},

						},			name: "should convert output section with authentication",			name: "should convert output section with authentication",

						OTLP: &telemetryv1beta1.OTLPInput{

							Disabled: boolPtr(true),			input: &MetricPipeline{			input: &MetricPipeline{

						},

					},				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

				},

			},					Output: MetricPipelineOutput{					Output: MetricPipelineOutput{

			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{						OTLP: &OTLPOutput{						OTLP: &OTLPOutput{

					Input: MetricPipelineInput{

						Prometheus: &MetricPipelinePrometheusInput{							Protocol: "grpc",							Protocol: "grpc",

							Enabled: boolPtr(true),

							Namespaces: &NamespaceSelector{							Endpoint: ValueType{							Endpoint: ValueType{

								Include: []string{"ns1"},

							},								Value: "localhost:4317",								Value: "localhost:4317",

						},

						Runtime: &MetricPipelineRuntimeInput{							},							},

							Enabled: boolPtr(true),

							Resources: &MetricPipelineRuntimeInputResources{							Authentication: &AuthenticationOptions{							Authentication: &AuthenticationOptions{

								Pod: &MetricPipelineRuntimeInputResource{

									Enabled: boolPtr(true),								Basic: &BasicAuthOptions{								Basic: &BasicAuthOptions{

								},

							},									User: ValueType{									User: ValueType{

						},

						Istio: &MetricPipelineIstioInput{										Value: "user",										Value: "user",

							Enabled: boolPtr(true),

							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{									},									},

								Enabled: boolPtr(true),

							},									Password: ValueType{									Password: ValueType{

						},

						OTLP: &OTLPInput{										Value: "pass",										Value: "pass",

							Disabled: boolPtr(true),

						},									},									},

					},

				},								},								},

			},

		},							},							},

		{

			name: "should convert output section with TLS",						},						},

			input: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{					},					},

					Output: telemetryv1beta1.MetricPipelineOutput{

						OTLP: &telemetryv1beta1.OTLPOutput{				},				},

							Protocol: "grpc",

							Endpoint: telemetryv1beta1.ValueType{			},			},

								Value: "localhost:4317",

							},			expected: &telemetryv1beta1.MetricPipeline{			expected: &telemetryv1beta1.MetricPipeline{

							TLS: &telemetryv1beta1.OutputTLS{

								Disabled: true,				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

								CA: &telemetryv1beta1.ValueType{

									Value: "ca-cert",					Output: telemetryv1beta1.MetricPipelineOutput{					Output: telemetryv1beta1.MetricPipelineOutput{

								},

								Cert: &telemetryv1beta1.ValueType{						OTLP: &telemetryv1beta1.OTLPOutput{						OTLP: &telemetryv1beta1.OTLPOutput{

									Value: "cert",

								},							Protocol: "grpc",							Protocol: "grpc",

								Key: &telemetryv1beta1.ValueType{

									Value: "key",							Endpoint: telemetryv1beta1.ValueType{							Endpoint: telemetryv1beta1.ValueType{

								},

							},								Value: "localhost:4317",								Value: "localhost:4317",

						},

					},							},							},

				},

			},							Authentication: &telemetryv1beta1.AuthenticationOptions{							Authentication: &telemetryv1beta1.AuthenticationOptions{

			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{								Basic: &telemetryv1beta1.BasicAuthOptions{								Basic: &telemetryv1beta1.BasicAuthOptions{

					Output: MetricPipelineOutput{

						OTLP: &OTLPOutput{									User: telemetryv1beta1.ValueType{									User: telemetryv1beta1.ValueType{

							Protocol: "grpc",

							Endpoint: ValueType{										Value: "user",										Value: "user",

								Value: "localhost:4317",

							},									},									},

							TLS: &OTLPTLS{

								Insecure: true,									Password: telemetryv1beta1.ValueType{									Password: telemetryv1beta1.ValueType{

								CA: &ValueType{

									Value: "ca-cert",										Value: "pass",										Value: "pass",

								},

								Cert: &ValueType{									},									},

									Value: "cert",

								},								},								},

								Key: &ValueType{

									Value: "key",							},							},

								},

							},						},						},

						},

					},					},					},

				},

			},				},				},

		},

	}			},			},



	for _, tt := range tests {		},		},

		t.Run(tt.name, func(t *testing.T) {

			dst := &MetricPipeline{}	}	}

			err := dst.ConvertFrom(tt.input)

			assert.NoError(t, err)

			assert.Equal(t, tt.expected, dst)

		})	for _, tt := range tests {	for _, tt := range tests {

	}

}		t.Run(tt.name, func(t *testing.T) {		t.Run(tt.name, func(t *testing.T) {



func TestMetricPipelineConvertToInvalidType(t *testing.T) {			dst := &telemetryv1beta1.MetricPipeline{}			dst := &telemetryv1beta1.MetricPipeline{}

	src := &MetricPipeline{}

	dst := &MetricPipeline{} // Wrong type, should be v1beta1.MetricPipeline			err := tt.input.ConvertTo(dst)			err := tt.input.ConvertTo(dst)

	err := src.ConvertTo(dst)

	assert.Error(t, err)			assert.NoError(t, err)			assert.NoError(t, err)

	assert.Equal(t, errDstTypeUnsupported, err)

}			assert.Equal(t, tt.expected, dst)			assert.Equal(t, tt.expected, dst)

		})		})

	}	}

}}



func TestMetricPipelineConvertFrom(t *testing.T) {func TestMetricPipelineConvertFrom(t *testing.T) {

	tests := []struct {	tests := []struct {

		name     string		name     string

		input    *telemetryv1beta1.MetricPipeline		input    *telemetryv1beta1.MetricPipeline

		expected *MetricPipeline		expected *MetricPipeline

	}{	}{

		{		{

			name: "should convert basic fields",			name: "should convert basic fields",

			input: &telemetryv1beta1.MetricPipeline{			input: &telemetryv1beta1.MetricPipeline{

				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

					Name:      "test-pipeline",					Name:      "test-pipeline",

					Namespace: "test-ns",					Namespace: "test-ns",

				},				},

				Spec: telemetryv1beta1.MetricPipelineSpec{},				Spec: telemetryv1beta1.MetricPipelineSpec{},

			},			},

			expected: &MetricPipeline{			expected: &MetricPipeline{

				ObjectMeta: metav1.ObjectMeta{				ObjectMeta: metav1.ObjectMeta{

					Name:      "test-pipeline",					Name:      "test-pipeline",

					Namespace: "test-ns",					Namespace: "test-ns",

				},				},

				Spec: MetricPipelineSpec{},				Spec: MetricPipelineSpec{},

			},			},

		},		},

		{		{

			name: "should convert namespaces in namespace selectors without validation",			name: "should convert namespaces in namespace selectors without validation",

			input: &telemetryv1beta1.MetricPipeline{			input: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

					Input: telemetryv1beta1.MetricPipelineInput{					Input: telemetryv1beta1.MetricPipelineInput{

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{

							Namespaces: &telemetryv1beta1.NamespaceSelector{							Namespaces: &telemetryv1beta1.NamespaceSelector{

								Include: []string{"valid-ns", "Invalid_NS"},								Include: []string{"valid-ns", "Invalid_NS"},

								Exclude: []string{"valid-excluded", "Invalid@NS"},								Exclude: []string{"valid-excluded", "Invalid@NS"},

							},							},

						},						},

					},					},

				},				},

			},			},

			expected: &MetricPipeline{			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

					Input: MetricPipelineInput{					Input: MetricPipelineInput{

						Runtime: &MetricPipelineRuntimeInput{						Runtime: &MetricPipelineRuntimeInput{

							Namespaces: &NamespaceSelector{							Namespaces: &NamespaceSelector{

								Include: []string{"valid-ns", "Invalid_NS"},								Include: []string{"valid-ns", "Invalid_NS"},

								Exclude: []string{"valid-excluded", "Invalid@NS"},								Exclude: []string{"valid-excluded", "Invalid@NS"},

							},							},

						},						},

					},					},

				},				},

			},			},

		},		},

		{		{

			name: "should convert all input sections",			name: "should convert all input sections",

			input: &telemetryv1beta1.MetricPipeline{			input: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

					Input: telemetryv1beta1.MetricPipelineInput{					Input: telemetryv1beta1.MetricPipelineInput{

						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{						Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{

							Enabled: boolPtr(true),							Enabled: true,

							Namespaces: &telemetryv1beta1.NamespaceSelector{							Namespaces: &telemetryv1beta1.NamespaceSelector{

								Include: []string{"ns1"},								Include: []string{"ns1"},

							},							},

						},						},

						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{

							Enabled: boolPtr(true),							Enabled: true,

							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{							Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{

								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{								Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{

									Enabled: boolPtr(true),									Enabled: true,

								},								},

							},							},

						},						},

						Istio: &telemetryv1beta1.MetricPipelineIstioInput{						Istio: &telemetryv1beta1.MetricPipelineIstioInput{

							Enabled: boolPtr(true),							Enabled: true,

							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{							DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{

								Enabled: boolPtr(true),								Enabled: true,

							},							},

						},						},

						OTLP: &telemetryv1beta1.OTLPInput{						OTLP: &telemetryv1beta1.OTLPInput{

							Disabled: boolPtr(true),							Disabled: true,

						},						},

					},					},

				},				},

			},			},

			expected: &MetricPipeline{			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

					Input: MetricPipelineInput{					Input: MetricPipelineInput{

						Prometheus: &MetricPipelinePrometheusInput{						Prometheus: &MetricPipelinePrometheusInput{

							Enabled: boolPtr(true),							Enabled: true,

							Namespaces: &NamespaceSelector{							Namespaces: &NamespaceSelector{

								Include: []string{"ns1"},								Include: []string{"ns1"},

							},							},

						},						},

						Runtime: &MetricPipelineRuntimeInput{						Runtime: &MetricPipelineRuntimeInput{

							Enabled: boolPtr(true),							Enabled: true,

							Resources: &MetricPipelineRuntimeInputResources{							Resources: &MetricPipelineRuntimeInputResources{

								Pod: &MetricPipelineRuntimeInputResource{								Pod: &MetricPipelineRuntimeInputResource{

									Enabled: boolPtr(true),									Enabled: true,

								},								},

							},							},

						},						},

						Istio: &MetricPipelineIstioInput{						Istio: &MetricPipelineIstioInput{

							Enabled: boolPtr(true),							Enabled: true,

							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{							DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{

								Enabled: boolPtr(true),								Enabled: true,

							},							},

						},						},

						OTLP: &OTLPInput{						OTLP: &OTLPInput{

							Disabled: boolPtr(true),							Disabled: true,

						},						},

					},					},

				},				},

			},			},

		},		},

		{		{

			name: "should convert output section with TLS",			name: "should convert output section with TLS",

			input: &telemetryv1beta1.MetricPipeline{			input: &telemetryv1beta1.MetricPipeline{

				Spec: telemetryv1beta1.MetricPipelineSpec{				Spec: telemetryv1beta1.MetricPipelineSpec{

					Output: telemetryv1beta1.MetricPipelineOutput{					Output: telemetryv1beta1.MetricPipelineOutput{

						OTLP: &telemetryv1beta1.OTLPOutput{						OTLP: &telemetryv1beta1.OTLPOutput{

							Protocol: "grpc",							Protocol: "grpc",

							Endpoint: telemetryv1beta1.ValueType{							Endpoint: telemetryv1beta1.ValueType{

								Value: "localhost:4317",								Value: "localhost:4317",

							},							},

							TLS: &telemetryv1beta1.OutputTLS{							TLS: &telemetryv1beta1.OutputTLS{

								Disabled: true,								Disabled: true,

								CA: &telemetryv1beta1.ValueType{								CA: &telemetryv1beta1.ValueType{

									Value: "ca-cert",									Value: "ca-cert",

								},								},

								Cert: &telemetryv1beta1.ValueType{								Cert: &telemetryv1beta1.ValueType{

									Value: "cert",									Value: "cert",

								},								},

								Key: &telemetryv1beta1.ValueType{								Key: &telemetryv1beta1.ValueType{

									Value: "key",									Value: "key",

								},								},

							},							},

						},						},

					},					},

				},				},

			},			},

			expected: &MetricPipeline{			expected: &MetricPipeline{

				Spec: MetricPipelineSpec{				Spec: MetricPipelineSpec{

					Output: MetricPipelineOutput{					Output: MetricPipelineOutput{

						OTLP: &OTLPOutput{						OTLP: &OTLPOutput{

							Protocol: "grpc",							Protocol: "grpc",

							Endpoint: ValueType{							Endpoint: ValueType{

								Value: "localhost:4317",								Value: "localhost:4317",

							},							},

							TLS: &OTLPTLS{							TLS: &OTLPTLS{

								Insecure: true,								Insecure: true,

								CA: &ValueType{								CA: &ValueType{

									Value: "ca-cert",									Value: "ca-cert",

								},								},

								Cert: &ValueType{								Cert: &ValueType{

									Value: "cert",									Value: "cert",

								},								},

								Key: &ValueType{								Key: &ValueType{

									Value: "key",									Value: "key",

								},								},

							},							},

						},						},

					},					},

				},				},

			},			},

		},		},

	}	}



	for _, tt := range tests {	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {		t.Run(tt.name, func(t *testing.T) {

			dst := &MetricPipeline{}			dst := &MetricPipeline{}

			err := dst.ConvertFrom(tt.input)			err := dst.ConvertFrom(tt.input)

			assert.NoError(t, err)			assert.NoError(t, err)

			assert.Equal(t, tt.expected, dst)			assert.Equal(t, tt.expected, dst)

		})		})

	}	}

}}



func TestMetricPipelineConvertToInvalidType(t *testing.T) {func TestMetricPipelineConvertToInvalidType(t *testing.T) {

	src := &MetricPipeline{}	src := &MetricPipeline{}

	dst := &MetricPipeline{} // Wrong type, should be v1beta1.MetricPipeline	dst := &MetricPipeline{} // Wrong type, should be v1beta1.MetricPipeline

	err := src.ConvertTo(dst)	err := src.ConvertTo(dst)

	assert.Error(t, err)	assert.Error(t, err)

	assert.Equal(t, errDstTypeUnsupported, err)	assert.Equal(t, errDstTypeUnsupported, err)

}}