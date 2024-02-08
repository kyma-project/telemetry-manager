package builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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
    multiline.parser docker, cri, go, python, java
    path             /var/log/containers/*.log
    read_from_head   true
    skip_long_lines  on
    storage.type     filesystem
    tag              test-logpipeline.*

`
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{},
		},
	}

	actual := createInputSection(logPipeline, includePath, exlucdePath)
	require.Equal(t, expected, actual)
}

func TestCreateIncludeAndExcludePath(t *testing.T) {
	var tests = []struct {
		name             string
		pipeline         *telemetryv1alpha1.LogPipeline
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"empty",
			&telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_compass-system_*-*.log",
			},
		},
		{
			"include system",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								System: true,
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
			},
		},
		{
			"include foo namespace",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Include: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_foo_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
			},
		},
		{
			"include foo container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Containers: telemetryv1alpha1.InputContainers{
								Include: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_foo-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_compass-system_*-*.log",
			},
		},
		{
			"include foo namespace and bar container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Include: []string{
									"foo",
								},
							},
							Containers: telemetryv1alpha1.InputContainers{
								Include: []string{
									"bar",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_foo_bar-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
			},
		},
		{
			"include foo and bar namespace, include istio-proxy container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Include: []string{
									"foo",
									"bar",
								},
							},
							Containers: telemetryv1alpha1.InputContainers{
								Include: []string{
									"istio-proxy",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_foo_istio-proxy-*.log",
				"/var/log/containers/*_bar_istio-proxy-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
			},
		},

		{
			"exclude foo namespace",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Exclude: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_foo_*-*.log",
			},
		},
		{
			"include system, exclude foo container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								System: true,
							},
							Containers: telemetryv1alpha1.InputContainers{
								Exclude: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_*_foo-*.log",
			},
		},
		{
			"exclude foo container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Containers: telemetryv1alpha1.InputContainers{
								Exclude: []string{
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_compass-system_*-*.log",
				"/var/log/containers/*_*_foo-*.log",
			},
		},
		{
			"exclude foo namespace, exclude bar container",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Exclude: []string{
									"foo",
								},
							},
							Containers: telemetryv1alpha1.InputContainers{
								Exclude: []string{
									"bar",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_*_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
				"/var/log/containers/*_foo_*-*.log",
				"/var/log/containers/*_*_bar-*.log",
			},
		},
		{
			"include system and foo namespaces",
			&telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.Input{
						Application: telemetryv1alpha1.ApplicationInput{
							Namespaces: telemetryv1alpha1.InputNamespaces{
								Include: []string{
									"kyma-system",
									"kube-system",
									"istio-system",
									"compass-system",
									"foo",
								},
							},
						},
					},
				},
			},
			[]string{
				"/var/log/containers/*_kyma-system_*-*.log",
				"/var/log/containers/*_kube-system_*-*.log",
				"/var/log/containers/*_istio-system_*-*.log",
				"/var/log/containers/*_compass-system_*-*.log",
				"/var/log/containers/*_foo_*-*.log",
			},
			[]string{
				"/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualIncludes := strings.Split(createIncludePath(test.pipeline), ",")
			require.Equal(t, test.expectedIncludes, actualIncludes, "Unexpected include paths for test: %s", test.name)

			actualExcludes := strings.Split(createExcludePath(test.pipeline), ",")
			require.Equal(t, test.expectedExcludes, actualExcludes, "Unexpected exclude paths for test: %s", test.name)
		})
	}
}
