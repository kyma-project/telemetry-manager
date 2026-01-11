package common

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
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
	require := require.New(t)

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
	expectedK8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}
	expectedExtractLabels := []ExtractLabel{
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
	}

	config := K8sAttributesProcessorConfig(&operatorv1beta1.EnrichmentSpec{
		ExtractPodLabels: []operatorv1beta1.PodLabel{
			{Key: "", KeyPrefix: "app.kubernetes.io/name"},
			{Key: "app", KeyPrefix: ""},
		},
	})

	require.Equal("serviceAccount", config.AuthType)
	require.Equal(false, config.Passthrough)
	require.Equal(expectedPodAssociations, config.PodAssociation, "PodAssociation should match")

	require.ElementsMatch(expectedK8sAttributes, config.Extract.Metadata, "Metadata should match")
	require.ElementsMatch(expectedExtractLabels, config.Extract.Labels, "Labels should match")
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
