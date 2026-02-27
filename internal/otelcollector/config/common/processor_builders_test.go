package common

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestInsertClusterAttributesProcessorStatements(t *testing.T) {
	require := require.New(t)

	expectedProcessorStatements := []TransformProcessorStatements{{
		Statements: []string{
			"set(resource.attributes[\"k8s.cluster.name\"], \"test-cluster\") where resource.attributes[\"k8s.cluster.name\"] == nil or resource.attributes[\"k8s.cluster.name\"] == \"\"",
			"set(resource.attributes[\"k8s.cluster.uid\"], \"test-cluster-uid\") where resource.attributes[\"k8s.cluster.uid\"] == nil or resource.attributes[\"k8s.cluster.uid\"] == \"\"",
			"set(resource.attributes[\"cloud.provider\"], \"test-cloud-provider\") where resource.attributes[\"cloud.provider\"] == nil or resource.attributes[\"cloud.provider\"] == \"\"",
		},
	}}

	processorStatements := InsertClusterAttributesProcessorStatements(
		ClusterOptions{
			ClusterName:   "test-cluster",
			ClusterUID:    "test-cluster-uid",
			CloudProvider: "test-cloud-provider",
		},
	)

	require.ElementsMatch(expectedProcessorStatements, processorStatements, "Attributes should match")
}

func TestInsertClusterAttributesProcessorStatementsWithEmptyValues(t *testing.T) {
	require := require.New(t)

	expectedProcessorStatements := []TransformProcessorStatements{{
		Statements: []string{
			"set(resource.attributes[\"k8s.cluster.name\"], \"\") where resource.attributes[\"k8s.cluster.name\"] == nil or resource.attributes[\"k8s.cluster.name\"] == \"\"",
			"set(resource.attributes[\"k8s.cluster.uid\"], \"\") where resource.attributes[\"k8s.cluster.uid\"] == nil or resource.attributes[\"k8s.cluster.uid\"] == \"\"",
		},
	}}

	processorStatements := InsertClusterAttributesProcessorStatements(
		ClusterOptions{
			ClusterName:   "",
			ClusterUID:    "",
			CloudProvider: "",
		},
	)

	require.ElementsMatch(expectedProcessorStatements, processorStatements, "Attributes should match")
}

func TestDropKymaAttributesProcessorStatements(t *testing.T) {
	require := require.New(t)

	expectedProcessorStatements := []TransformProcessorStatements{{
		Statements: []string{
			"delete_matching_keys(resource.attributes, \"kyma.*\")",
		},
	}}

	processorStatements := DropKymaAttributesProcessorStatements()

	require.ElementsMatch(expectedProcessorStatements, processorStatements, "Attributes should match")
}

func TestDropUnknownServiceNameProcessorStatements(t *testing.T) {
	require := require.New(t)

	expectedProcessorStatements := []TransformProcessorStatements{{
		Statements: []string{
			"delete_key(resource.attributes, \"service.name\") where resource.attributes[\"service.name\"] != nil and HasPrefix(resource.attributes[\"service.name\"], \"unknown_service\")",
		},
	}}

	processorStatements := DropUnknownServiceNameProcessorStatements()

	require.ElementsMatch(expectedProcessorStatements, processorStatements, "Attributes should match")
}

func TestTransformedInstrumentationScope(t *testing.T) {
	instrumentationScopeVersion := "main"
	tests := []struct {
		name        string
		want        *TransformProcessor
		inputSource InputSourceType
	}{
		{
			name: "InputSourceRuntime",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []TransformProcessorStatements{{
					Statements: []string{
						"set(scope.version, \"main\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver\"",
						"set(scope.name, \"io.kyma-project.telemetry/runtime\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver\"",
					},
				}},
			},
			inputSource: InputSourceRuntime,
		}, {
			name: "InputSourcePrometheus",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []TransformProcessorStatements{
					{
						Statements: []string{
							"set(scope.version, \"main\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"",
							"set(scope.name, \"io.kyma-project.telemetry/prometheus\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"",
						},
					},
				},
			},
			inputSource: InputSourcePrometheus,
		}, {
			name: "InputSourceIstio",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []TransformProcessorStatements{{
					Statements: []string{
						"set(scope.version, \"main\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"",
						"set(scope.name, \"io.kyma-project.telemetry/istio\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"",
					},
				}},
			},
			inputSource: InputSourceIstio,
		}, {
			name: "InputSourceKyma",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []TransformProcessorStatements{{
					Statements: []string{
						"set(scope.version, \"main\") where scope.name == \"github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver\"",
						"set(scope.name, \"io.kyma-project.telemetry/kyma\") where scope.name == \"github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver\"",
					},
				}},
			},
			inputSource: InputSourceKyma,
		}, {
			name: "InputSourceK8sCluster",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []TransformProcessorStatements{{
					Statements: []string{
						"set(scope.version, \"main\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver\"",
						"set(scope.name, \"io.kyma-project.telemetry/runtime\") where scope.name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver\"",
					},
				}},
			},
			inputSource: InputSourceK8sCluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InstrumentationScopeProcessorConfig(instrumentationScopeVersion, tt.inputSource); !compareTransformProcessor(got, tt.want) {
				t.Errorf("makeInstrumentationScopeProcessor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func compareTransformProcessor(got, want *TransformProcessor) bool {
	if got.ErrorMode != want.ErrorMode {
		return false
	}

	if len(got.MetricStatements) != len(want.MetricStatements) {
		return false
	}

	for i, statement := range got.MetricStatements {
		if len(statement.Statements) != len(want.MetricStatements[i].Statements) {
			return false
		}

		for j, s := range statement.Statements {
			if s != want.MetricStatements[i].Statements[j] {
				return false
			}
		}
	}

	return true
}

func TestK8sAttributesProcessorConfig(t *testing.T) {
	expectedPodAssociations := []PodAssociations{
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.ip"}},
		},
		{
			Sources: []PodAssociation{{From: "resource_attribute", Name: "k8s.pod.uid"}},
		},
		{
			Sources: []PodAssociation{{From: "connection"}},
		},
	}

	tests := []struct {
		name                  string
		isOTel                bool
		expectedK8sAttributes []string
		expectedExtractLabels []ExtractLabel
	}{
		{
			name:   "kyma-legacy",
			isOTel: false,
			expectedK8sAttributes: []string{
				"k8s.pod.name",
				"k8s.node.name",
				"k8s.namespace.name",
				"k8s.deployment.name",
				"k8s.statefulset.name",
				"k8s.daemonset.name",
				"k8s.cronjob.name",
				"k8s.job.name",
			},
			expectedExtractLabels: []ExtractLabel{
				{
					From:    "pod",
					Key:     "app.kubernetes.io/name",
					TagName: "kyma.kubernetes_io_app_name",
				},
				{
					From:    "pod",
					Key:     "app",
					TagName: "kyma.app_name",
				},
				{
					From:    "node",
					Key:     "topology.kubernetes.io/region",
					TagName: "cloud.region",
				},
				{
					From:    "node",
					Key:     "topology.kubernetes.io/zone",
					TagName: "cloud.availability_zone",
				},
				{
					From:    "node",
					Key:     "node.kubernetes.io/instance-type",
					TagName: "host.type",
				},
				{
					From:    "node",
					Key:     "kubernetes.io/arch",
					TagName: "host.arch",
				},
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io/name.*)",
					TagName:  "k8s.pod.label.$0",
				},
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name:   "otel",
			isOTel: true,
			expectedK8sAttributes: []string{
				"k8s.pod.name",
				"k8s.node.name",
				"k8s.namespace.name",
				"k8s.deployment.name",
				"k8s.statefulset.name",
				"k8s.daemonset.name",
				"k8s.cronjob.name",
				"k8s.job.name",
				"service.namespace",
				"service.name",
				"service.version",
				"service.instance.id",
			},
			expectedExtractLabels: []ExtractLabel{
				{
					From:    "node",
					Key:     "topology.kubernetes.io/region",
					TagName: "cloud.region",
				},
				{
					From:    "node",
					Key:     "topology.kubernetes.io/zone",
					TagName: "cloud.availability_zone",
				},
				{
					From:    "node",
					Key:     "node.kubernetes.io/instance-type",
					TagName: "host.type",
				},
				{
					From:    "node",
					Key:     "kubernetes.io/arch",
					TagName: "host.arch",
				},
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io/name.*)",
					TagName:  "k8s.pod.label.$0",
				},
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := K8sAttributesProcessorConfig(&operatorv1beta1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1beta1.PodLabel{
					{Key: "", KeyPrefix: "app.kubernetes.io/name"},
					{Key: "app", KeyPrefix: ""},
				},
			}, tt.isOTel)

			require.Equal(t, "serviceAccount", config.AuthType)
			require.Equal(t, false, config.Passthrough)
			require.Equal(t, expectedPodAssociations, config.PodAssociation, "PodAssociation should match")

			require.ElementsMatch(t, tt.expectedK8sAttributes, config.Extract.Metadata, "Metadata should match")
			require.ElementsMatch(t, tt.expectedExtractLabels, config.Extract.Labels, "Labels should match")
		})
	}
}

func TestBuildPodLabelEnrichments(t *testing.T) {
	tests := []struct {
		name     string
		presets  *operatorv1beta1.EnrichmentSpec
		expected []ExtractLabel
	}{
		{
			name: "Enrichments disabled",
			presets: &operatorv1beta1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1beta1.PodLabel{},
			},
			expected: []ExtractLabel{},
		},
		{
			name: "Enrichments enabled with key",
			presets: &operatorv1beta1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1beta1.PodLabel{
					{Key: "app"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with key prefix",
			presets: &operatorv1beta1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1beta1.PodLabel{
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io.*)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
		{
			name: "Enrichments enabled with multiple labels",
			presets: &operatorv1beta1.EnrichmentSpec{
				ExtractPodLabels: []operatorv1beta1.PodLabel{
					{Key: "app"},
					{KeyPrefix: "app.kubernetes.io"},
				},
			},
			expected: []ExtractLabel{
				{
					From:     "pod",
					KeyRegex: "(^app$)",
					TagName:  "k8s.pod.label.$0",
				},
				{
					From:     "pod",
					KeyRegex: "(app.kubernetes.io.*)",
					TagName:  "k8s.pod.label.$0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			result := extractPodLabels(tt.presets)
			require.ElementsMatch(tt.expected, result)
		})
	}
}

func TestKymaInputNameProcessorStatements(t *testing.T) {
	type args struct {
		inputSource InputSourceType
	}

	tests := []struct {
		name string
		args args
		want []TransformProcessorStatements
	}{
		{
			name: "InputSourceRuntime",
			args: args{inputSource: InputSourceRuntime},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"runtime\")",
				},
			}},
		},
		{
			name: "InputSourcePrometheus",
			args: args{inputSource: InputSourcePrometheus},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"prometheus\")",
				},
			}},
		},
		{
			name: "InputSourceIstio",
			args: args{inputSource: InputSourceIstio},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"istio\")",
				},
			}},
		},
		{
			name: "InputSourceOTLP",
			args: args{inputSource: InputSourceOTLP},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"otlp\")",
				},
			}},
		},
		{
			name: "InputSourceKyma",
			args: args{inputSource: InputSourceKyma},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"kyma\")",
				},
			}},
		},
		{
			name: "InputSourceK8sCluster",
			args: args{inputSource: InputSourceK8sCluster},
			want: []TransformProcessorStatements{{
				Statements: []string{
					"set(resource.attributes[\"kyma.input.name\"], \"k8s_cluster\")",
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KymaInputNameProcessorStatements(tt.args.inputSource); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KymaInputNameProcessorStatements() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveServiceNameConfig(t *testing.T) {
	require := require.New(t)

	config := ResolveServiceNameConfig()

	require.NotNil(config)
	require.Equal([]string{"kyma.kubernetes_io_app_name", "kyma.app_name"}, config.ResourceAttributes)
}

func TestLogFilterProcessorConfig(t *testing.T) {
	require := require.New(t)

	logs := FilterProcessorLogs{
		Log: []string{"condition1", "condition2"},
	}

	config := LogFilterProcessorConfig(logs)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(logs, config.Logs)
	require.Empty(config.Metrics.Metric)
	require.Empty(config.Metrics.Datapoint)
	require.Empty(config.Traces.Span)
}

func TestMetricFilterProcessorConfig(t *testing.T) {
	require := require.New(t)

	metrics := FilterProcessorMetrics{
		Metric:    []string{"metric_condition1"},
		Datapoint: []string{"datapoint_condition1"},
	}

	config := MetricFilterProcessorConfig(metrics)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(metrics, config.Metrics)
	require.Empty(config.Logs.Log)
	require.Empty(config.Traces.Span)
}

func TestTraceFilterProcessorConfig(t *testing.T) {
	require := require.New(t)

	traces := FilterProcessorTraces{
		Span:      []string{"span_condition1"},
		SpanEvent: []string{"spanevent_condition1"},
	}

	config := TraceFilterProcessorConfig(traces)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(traces, config.Traces)
	require.Empty(config.Logs.Log)
	require.Empty(config.Metrics.Metric)
}

func TestFilterSpecsToLogFilterProcessorConfig(t *testing.T) {
	tests := []struct {
		name               string
		specs              []telemetryv1beta1.FilterSpec
		expectedConditions []string
	}{
		{
			name:               "empty specs",
			specs:              []telemetryv1beta1.FilterSpec{},
			expectedConditions: nil,
		},
		{
			name: "single spec with single condition",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"body == \"test\""}},
			},
			expectedConditions: []string{"body == \"test\""},
		},
		{
			name: "single spec with multiple conditions",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1", "condition2"}},
			},
			expectedConditions: []string{"condition1", "condition2"},
		},
		{
			name: "multiple specs with conditions",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1"}},
				{Conditions: []string{"condition2", "condition3"}},
			},
			expectedConditions: []string{"condition1", "condition2", "condition3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			config := FilterSpecsToLogFilterProcessorConfig(tt.specs)

			require.NotNil(config)
			require.Equal("ignore", config.ErrorMode)
			require.Equal(tt.expectedConditions, config.Logs.Log)
		})
	}
}

func TestFilterSpecsToMetricFilterProcessorConfig(t *testing.T) {
	tests := []struct {
		name               string
		specs              []telemetryv1beta1.FilterSpec
		expectedConditions []string
	}{
		{
			name:               "empty specs",
			specs:              []telemetryv1beta1.FilterSpec{},
			expectedConditions: nil,
		},
		{
			name: "single spec with single condition",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"metric.name == \"test\""}},
			},
			expectedConditions: []string{"metric.name == \"test\""},
		},
		{
			name: "multiple specs with conditions",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1"}},
				{Conditions: []string{"condition2"}},
			},
			expectedConditions: []string{"condition1", "condition2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			config := FilterSpecsToMetricFilterProcessorConfig(tt.specs)

			require.NotNil(config)
			require.Equal("ignore", config.ErrorMode)
			require.Equal(tt.expectedConditions, config.Metrics.Datapoint)
		})
	}
}

func TestFilterSpecsToTraceFilterProcessorConfig(t *testing.T) {
	tests := []struct {
		name               string
		specs              []telemetryv1beta1.FilterSpec
		expectedConditions []string
	}{
		{
			name:               "empty specs",
			specs:              []telemetryv1beta1.FilterSpec{},
			expectedConditions: nil,
		},
		{
			name: "single spec with single condition",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"span.name == \"test\""}},
			},
			expectedConditions: []string{"span.name == \"test\""},
		},
		{
			name: "multiple specs with conditions",
			specs: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1"}},
				{Conditions: []string{"condition2"}},
			},
			expectedConditions: []string{"condition1", "condition2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			config := FilterSpecsToTraceFilterProcessorConfig(tt.specs)

			require.NotNil(config)
			require.Equal("ignore", config.ErrorMode)
			require.Equal(tt.expectedConditions, config.Traces.Span)
		})
	}
}

func TestLogTransformProcessorConfig(t *testing.T) {
	require := require.New(t)

	statements := []TransformProcessorStatements{
		{
			Statements: []string{"set(body, \"test\")"},
			Conditions: []string{"body != nil"},
		},
	}

	config := LogTransformProcessorConfig(statements)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(statements, config.LogStatements)
	require.Empty(config.MetricStatements)
	require.Empty(config.TraceStatements)
}

func TestMetricTransformProcessorConfig(t *testing.T) {
	require := require.New(t)

	statements := []TransformProcessorStatements{
		{
			Statements: []string{"set(name, \"test\")"},
		},
	}

	config := MetricTransformProcessorConfig(statements)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(statements, config.MetricStatements)
	require.Empty(config.LogStatements)
	require.Empty(config.TraceStatements)
}

func TestTraceTransformProcessorConfig(t *testing.T) {
	require := require.New(t)

	statements := []TransformProcessorStatements{
		{
			Statements: []string{"set(name, \"test\")"},
		},
	}

	config := TraceTransformProcessorConfig(statements)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Equal(statements, config.TraceStatements)
	require.Empty(config.LogStatements)
	require.Empty(config.MetricStatements)
}

func TestTransformSpecsToProcessorStatements(t *testing.T) {
	tests := []struct {
		name     string
		specs    []telemetryv1beta1.TransformSpec
		expected []TransformProcessorStatements
	}{
		{
			name:     "empty specs",
			specs:    []telemetryv1beta1.TransformSpec{},
			expected: []TransformProcessorStatements{},
		},
		{
			name: "single spec without conditions",
			specs: []telemetryv1beta1.TransformSpec{
				{
					Statements: []string{"set(body, \"modified\")"},
				},
			},
			expected: []TransformProcessorStatements{
				{
					Statements: []string{"set(body, \"modified\")"},
					Conditions: nil,
				},
			},
		},
		{
			name: "single spec with conditions",
			specs: []telemetryv1beta1.TransformSpec{
				{
					Statements: []string{"set(body, \"modified\")"},
					Conditions: []string{"body != nil"},
				},
			},
			expected: []TransformProcessorStatements{
				{
					Statements: []string{"set(body, \"modified\")"},
					Conditions: []string{"body != nil"},
				},
			},
		},
		{
			name: "multiple specs",
			specs: []telemetryv1beta1.TransformSpec{
				{
					Statements: []string{"statement1"},
					Conditions: []string{"condition1"},
				},
				{
					Statements: []string{"statement2", "statement3"},
					Conditions: []string{"condition2", "condition3"},
				},
			},
			expected: []TransformProcessorStatements{
				{
					Statements: []string{"statement1"},
					Conditions: []string{"condition1"},
				},
				{
					Statements: []string{"statement2", "statement3"},
					Conditions: []string{"condition2", "condition3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			result := TransformSpecsToProcessorStatements(tt.specs)

			require.Equal(tt.expected, result)
		})
	}
}

func TestInstrumentationScopeProcessorConfigMultipleSources(t *testing.T) {
	require := require.New(t)

	instrumentationScopeVersion := "v1.0.0"
	config := InstrumentationScopeProcessorConfig(instrumentationScopeVersion, InputSourceRuntime, InputSourcePrometheus)

	require.NotNil(config)
	require.Equal("ignore", config.ErrorMode)
	require.Len(config.MetricStatements, 1)
	require.Len(config.MetricStatements[0].Statements, 4)
}
