package builder

import (
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

func TestCreateInputWithIncludePath(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{},
		},
	}

	actual := createIncludePath(logPipeline)
	require.Equal(t, "/var/log/containers/*_*_*-*.log", actual)
}

func TestCreateInputWithExcludePath(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "test-logpipeline"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.Input{
				Application: telemetryv1alpha1.ApplicationInput{
					Namespaces: telemetryv1alpha1.InputNamespaces{
						System: true,
					},
				},
			},
		},
	}

	actual := createExcludePath(logPipeline)
	require.Equal(t, "/var/log/containers/telemetry-fluent-bit-*_kyma-system_fluent-bit-*.log", actual)
}
