package builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestCreateInput(t *testing.T) {
	includePath := "/var/log/containers/*.log"
	exlucdePath := "/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log"
	expected := `[INPUT]
    name             tail
    alias            test-logpipeline
    db               /data/flb_test-logpipeline.db
    exclude_path     /var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log
    mem_buf_limit    5MB
    multiline.parser cri
    path             /var/log/containers/*.log
    read_from_head   true
    skip_long_lines  on
    storage.type     filesystem
    tag              test-logpipeline.*

`
	logPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Input: telemetryv1beta1.LogPipelineInput{},
		},
	}

	actual := createInputSection(logPipeline, includePath, exlucdePath)
	require.Equal(t, expected, actual)
}

func TestCreateIncludeAndExcludePath(t *testing.T) {
	var tests = []struct {
		name             string
		pipeline         *telemetryv1beta1.LogPipeline
		collectAgentLogs bool
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"empty",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
			},
		},
		{
			"include agent logs",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
			},
			true,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
			},
		},
		{
			"include foo namespace",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_foo_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},
		{
			"include foo container",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Include: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_foo-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
			},
		},
		{
			"include foo namespace and bar container",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{
									"foo",
								},
							},
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Include: []string{
									"bar",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_foo_bar-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},
		{
			"include foo and bar namespace, include istio-proxy container",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{
									"foo",
									"bar",
								},
							},
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Include: []string{
									"istio-proxy",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_foo_istio-proxy-*.log",
				"/var/log/containers/*_bar_istio-proxy-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},

		{
			"exclude foo namespace",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_foo_*-*.log",
			},
		},
		{
			"exclude foo container",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Exclude: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_*_foo-*.log",
			},
		},
		{
			"exclude foo namespace, exclude bar container",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{
									"foo",
								},
							},
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Exclude: []string{
									"bar",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_foo_*-*.log",
				"/var/log/containers/*_*_bar-*.log",
			},
		},
		{
			"include system and foo namespaces",
			&telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{
									"kyma-system",
									"kube-system",
									"istio-system",
									"foo",
								},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_foo_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},
		{
			"empty include namespace list",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log", // should fall back to wildcard
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},
		{
			"empty include container list",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Include: []string{},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log", // should fall back to wildcard
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
			},
		},
		{
			"empty exclude namespace list - should include system namespaces",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				// Only agent exclusions, NO system namespace exclusions
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
			},
		},
		{
			"empty exclude container list",
			&telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Containers: &telemetryv1beta1.LogPipelineContainerSelector{
								Exclude: []string{},
							},
						},
					},
				},
			},
			false,
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				// System namespaces still excluded (default behavior)
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*system-logs-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*system-logs-collector-*_kyma-system_collector-*.log",
				"/var/log/containers/telemetry-log-agent-*_kyma-system_collector-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualIncludes := strings.Split(createIncludePath(test.pipeline), ",")
			require.Equal(t, test.expectedIncludes, actualIncludes, "Unexpected include paths for test: %s", test.name)

			actualExcludes := strings.Split(createExcludePath(test.pipeline, test.collectAgentLogs), ",")
			require.Equal(t, test.expectedExcludes, actualExcludes, "Unexpected exclude paths for test: %s", test.name)
		})
	}
}
